package rlm

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// StructuredCompletion executes a structured completion with schema validation
func (r *RLM) StructuredCompletion(query string, context string, config *StructuredConfig) (map[string]interface{}, RLMStats, error) {
	ctx := r.observer.StartSpan("rlm.structured_completion", map[string]string{
		"query_length":   fmt.Sprintf("%d", len(query)),
		"context_length": fmt.Sprintf("%d", len(context)),
	})
	defer r.observer.EndSpan(ctx)

	if config == nil || config.Schema == nil {
		return nil, RLMStats{}, fmt.Errorf("structured config and schema are required")
	}

	r.observer.Debug("structured", "Schema type: %s", config.Schema.Type)

	// Set defaults
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	// Apply meta-agent optimization for structured queries if enabled
	if r.metaAgent != nil {
		optimized, err := r.metaAgent.OptimizeForStructured(query, context, config.Schema)
		if err == nil && optimized != "" {
			r.observer.Debug("structured", "Using meta-agent optimized query for structured extraction")
			query = optimized
		}
	}

	// Create schema validator using Google's jsonschema-go for enhanced validation
	validator, validatorErr := NewSchemaValidator(config.Schema)
	if validatorErr != nil {
		r.observer.Debug("structured", "Schema validator creation info: %v (using fallback)", validatorErr)
	}
	_ = validator // Available for enhanced validation in parseAndValidateJSON

	// Decompose schema into sub-tasks
	subTasks := decomposeSchema(config.Schema)
	r.observer.Debug("structured", "Schema decomposed into %d subtasks", len(subTasks))

	// If simple schema or parallel disabled, use direct method
	if len(subTasks) <= 2 || !config.ParallelExecution {
		r.observer.Debug("structured", "Using direct completion method")
		return r.structuredCompletionDirect(query, context, config)
	}

	// Execute with parallel goroutines, with fallback to direct
	r.observer.Debug("structured", "Using parallel completion with %d subtasks", len(subTasks))
	result, stats, err := r.structuredCompletionParallel(query, context, config, subTasks)
	if err != nil {
		// Fallback to direct (single-call) method when parallel fails
		r.observer.Debug("structured", "Parallel execution failed (%v), falling back to direct method", err)
		return r.structuredCompletionDirect(query, context, config)
	}
	return result, stats, nil
}

// structuredCompletionDirect performs a single structured completion
func (r *RLM) structuredCompletionDirect(query string, context string, config *StructuredConfig) (map[string]interface{}, RLMStats, error) {
	schemaJSON, _ := json.Marshal(config.Schema)

	// Build comprehensive prompt with context and schema
	constraints := generateSchemaConstraints(config.Schema)
	requiredFieldsHint := ""
	if config.Schema.Type == "object" && len(config.Schema.Required) > 0 {
		requiredFieldsHint = fmt.Sprintf("\nREQUIRED FIELDS (you MUST include these): %s\n", strings.Join(config.Schema.Required, ", "))
	}

	prompt := fmt.Sprintf(
		"You are a data extraction assistant. Extract information from the context and return it as JSON.\n\n"+
		"Context:\n%s\n\n"+
		"Task: %s\n\n"+
		"Required JSON Schema:\n%s%s\n\n"+
		"%s"+
		"CRITICAL INSTRUCTIONS:\n"+
		"1. Return ONLY valid JSON - no explanations, no markdown, no code blocks\n"+
		"2. The JSON must match the schema EXACTLY\n"+
		"3. Include ALL required fields (see list above)\n"+
		"4. Use correct data types (strings in quotes, numbers without quotes, arrays in [], objects in {})\n"+
		"5. For arrays, return actual JSON arrays [] not objects\n"+
		"6. For enum fields, use ONLY the EXACT values listed - do not paraphrase or substitute\n"+
		"7. For nested objects, ensure ALL required fields within those objects are included\n"+
		"8. Start your response directly with { or [ depending on the schema\n\n"+
		"JSON Response:",
		context, query, string(schemaJSON), requiredFieldsHint, constraints,
	)

	var lastErr error
	stats := RLMStats{Depth: r.currentDepth}

	// Initialize messages for first attempt
	messages := []Message{
		{Role: "system", Content: "You are a data extraction assistant. Respond only with valid JSON objects."},
		{Role: "user", Content: prompt},
	}

	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		result, err := r.callLLM(messages)
		stats.LlmCalls++
		stats.Iterations++

		if err != nil {
			lastErr = err
			continue
		}

		parsed, err := parseAndValidateJSON(result, config.Schema)
		if err != nil {
			lastErr = err
			if attempt < config.MaxRetries-1 {
				// Build detailed validation feedback similar to Instructor
				validationFeedback := buildValidationFeedback(err, config.Schema, result)

				// Update messages with previous attempt and validation feedback for retry
				messages = append(messages,
					Message{Role: "assistant", Content: result},
					Message{Role: "user", Content: validationFeedback},
				)
			}
			continue
		}

		stats.ParsingRetries = attempt
		return parsed, stats, nil
	}

	return nil, stats, fmt.Errorf("failed to get valid structured output after %d attempts: %v", config.MaxRetries, lastErr)
}

// structuredCompletionParallel executes sub-tasks in parallel
func (r *RLM) structuredCompletionParallel(query string, context string, config *StructuredConfig, subTasks []SubTask) (map[string]interface{}, RLMStats, error) {
	results := make(map[string]interface{})
	var resultsMutex sync.Mutex

	var wg sync.WaitGroup
	errors := make([]error, len(subTasks))

	totalStats := RLMStats{}
	var statsMutex sync.Mutex

	for i, task := range subTasks {
		wg.Add(1)
		go func(idx int, t SubTask) {
			defer wg.Done()

			fieldName := strings.TrimPrefix(t.ID, "field_")

			// Wrap the sub-schema in an object wrapper so the LLM always
			// returns a JSON object with a predictable key. This eliminates
			// ambiguity for non-object field types (string, number, array, etc.)
			wrappedSchema := wrapFieldSchema(fieldName, t.Schema)

			taskQuery := fmt.Sprintf("%s\n\nSpecific focus: %s", query, t.Query)
			taskConfig := &StructuredConfig{
				Schema:            wrappedSchema,
				ParallelExecution: false, // Disable nested parallelization
				MaxRetries:        config.MaxRetries,
			}

			result, stats, err := r.structuredCompletionDirect(taskQuery, context, taskConfig)
			if err != nil {
				errors[idx] = fmt.Errorf("task %s failed: %w", t.ID, err)
				return
			}

			resultsMutex.Lock()
			// Extract the field value from the wrapper object
			if val, ok := result[fieldName]; ok {
				results[fieldName] = val
			} else {
				// Fallback: if the LLM didn't use the wrapper key, try __value__ or the result itself
				if val, ok := result["__value__"]; ok {
					results[fieldName] = val
				} else if len(result) == 1 {
					// Single-key result, use whatever value is there
					for _, v := range result {
						results[fieldName] = v
					}
				} else {
					// Use the entire result map as the field value (for object-typed fields)
					results[fieldName] = result
				}
			}
			resultsMutex.Unlock()

			statsMutex.Lock()
			totalStats.LlmCalls += stats.LlmCalls
			totalStats.Iterations += stats.Iterations
			if stats.Depth > totalStats.Depth {
				totalStats.Depth = stats.Depth
			}
			totalStats.ParsingRetries += stats.ParsingRetries
			statsMutex.Unlock()
		}(i, task)
	}

	wg.Wait()

	// Collect all errors
	var allErrors []string
	for _, err := range errors {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}
	if len(allErrors) > 0 {
		return nil, totalStats, fmt.Errorf("parallel execution failed (%d/%d tasks): %s",
			len(allErrors), len(subTasks), strings.Join(allErrors, "; "))
	}

	// Validate merged result against full schema
	if err := validateAgainstSchema(results, config.Schema); err != nil {
		return nil, totalStats, fmt.Errorf("merged result validation failed: %w", err)
	}

	return results, totalStats, nil
}

// wrapFieldSchema wraps a field's schema inside an object schema with a single
// key matching the field name. This ensures the LLM always returns a JSON object
// with a predictable structure, avoiding ambiguity for non-object fields.
//
// For example, a field "sentimentValue" with schema {type: "number"} becomes:
//
//	{type: "object", properties: {"sentimentValue": {type: "number"}}, required: ["sentimentValue"]}
func wrapFieldSchema(fieldName string, schema *JSONSchema) *JSONSchema {
	return &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			fieldName: schema,
		},
		Required: []string{fieldName},
	}
}

// decomposeSchema breaks down a schema into independent sub-tasks
func decomposeSchema(schema *JSONSchema) []SubTask {
	var subTasks []SubTask

	if schema.Type != "object" || schema.Properties == nil {
		return subTasks
	}

	for fieldName, fieldSchema := range schema.Properties {
		taskID := fmt.Sprintf("field_%s", fieldName)
		query := generateFieldQuery(fieldName, fieldSchema)

		subTasks = append(subTasks, SubTask{
			ID:           taskID,
			Query:        query,
			Schema:       fieldSchema,
			Dependencies: []string{},
			Path:         []string{fieldName},
		})
	}

	return subTasks
}

// generateSchemaConstraints creates human-readable constraint descriptions
func generateSchemaConstraints(schema *JSONSchema) string {
	var constraints []string

	if schema.Type == "object" && schema.Properties != nil {
		for fieldName, fieldSchema := range schema.Properties {
			// Number constraints with min/max
			if fieldSchema.Type == "number" || fieldSchema.Type == "integer" {
				var constraintParts []string
				if fieldSchema.Minimum != nil {
					constraintParts = append(constraintParts, fmt.Sprintf(">= %v", *fieldSchema.Minimum))
				}
				if fieldSchema.Maximum != nil {
					constraintParts = append(constraintParts, fmt.Sprintf("<= %v", *fieldSchema.Maximum))
				}
				if len(constraintParts) > 0 {
					constraints = append(constraints, fmt.Sprintf("- %s must be %s (%s)", fieldName, fieldSchema.Type, strings.Join(constraintParts, " and ")))
				}
			}

			// String constraints
			if fieldSchema.Type == "string" {
				var stringConstraints []string
				if fieldSchema.MinLength != nil {
					stringConstraints = append(stringConstraints, fmt.Sprintf("minLength: %d", *fieldSchema.MinLength))
				}
				if fieldSchema.MaxLength != nil {
					stringConstraints = append(stringConstraints, fmt.Sprintf("maxLength: %d", *fieldSchema.MaxLength))
				}
				if fieldSchema.Format != "" {
					stringConstraints = append(stringConstraints, fmt.Sprintf("format: %s", fieldSchema.Format))
				}
				if len(stringConstraints) > 0 {
					constraints = append(constraints, fmt.Sprintf("- %s must be string (%s)", fieldName, strings.Join(stringConstraints, ", ")))
				}
			}

			if len(fieldSchema.Enum) > 0 {
				constraints = append(constraints, fmt.Sprintf("- %s must be EXACTLY one of these values: %s (use these exact strings, do not modify)", fieldName, strings.Join(fieldSchema.Enum, ", ")))
			}

			// Array constraints
			if fieldSchema.Type == "array" {
				var arrayConstraints []string
				if fieldSchema.MinItems != nil {
					arrayConstraints = append(arrayConstraints, fmt.Sprintf("minItems: %d", *fieldSchema.MinItems))
				}
				if fieldSchema.MaxItems != nil {
					arrayConstraints = append(arrayConstraints, fmt.Sprintf("maxItems: %d", *fieldSchema.MaxItems))
				}
				if len(arrayConstraints) > 0 {
					constraints = append(constraints, fmt.Sprintf("- %s must be JSON array (%s)", fieldName, strings.Join(arrayConstraints, ", ")))
				} else {
					constraints = append(constraints, fmt.Sprintf("- %s must be a JSON array []", fieldName))
				}
			}
			// Add constraint for nested objects with required fields
			if fieldSchema.Type == "object" && len(fieldSchema.Required) > 0 {
				constraints = append(constraints, fmt.Sprintf("- %s must be an object with these REQUIRED fields: %s", fieldName, strings.Join(fieldSchema.Required, ", ")))
			}
		}
	}

	// Check nested array items for constraints
	if schema.Type == "array" && schema.Items != nil {
		if schema.Items.Type == "object" && schema.Items.Properties != nil {
			for fieldName, fieldSchema := range schema.Items.Properties {
				// Number constraints in array items
				if fieldSchema.Type == "number" || fieldSchema.Type == "integer" {
					var constraintParts []string
					if fieldSchema.Minimum != nil {
						constraintParts = append(constraintParts, fmt.Sprintf(">= %v", *fieldSchema.Minimum))
					}
					if fieldSchema.Maximum != nil {
						constraintParts = append(constraintParts, fmt.Sprintf("<= %v", *fieldSchema.Maximum))
					}
					if len(constraintParts) > 0 {
						constraints = append(constraints, fmt.Sprintf("- Each item's %s must be %s (%s)", fieldName, fieldSchema.Type, strings.Join(constraintParts, " and ")))
					}
				}

				if len(fieldSchema.Enum) > 0 {
					constraints = append(constraints, fmt.Sprintf("- Each item's %s must be EXACTLY one of these values: %s (copy exactly, do not modify these strings)", fieldName, strings.Join(fieldSchema.Enum, ", ")))
				}
			}
		}
	}

	if len(constraints) > 0 {
		return "CONSTRAINTS:\n" + strings.Join(constraints, "\n") + "\n\n"
	}
	return ""
}

// generateFieldQuery creates a focused query for a specific field based on its schema
func generateFieldQuery(fieldName string, schema *JSONSchema) string {
	var queryParts []string

	// Start with field name
	queryParts = append(queryParts, fmt.Sprintf("Extract the '%s' field from the conversation.", fieldName))

	// Add type-specific instructions
	switch schema.Type {
	case "object":
		if len(schema.Required) > 0 {
			fieldDetails := make([]string, 0, len(schema.Required))
			for _, reqField := range schema.Required {
				if propSchema, exists := schema.Properties[reqField]; exists {
					fieldDetails = append(fieldDetails, fmt.Sprintf("'%s' (%s)", reqField, propSchema.Type))
				}
			}
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with the key '%s' containing an object with these REQUIRED fields: %s.", fieldName, strings.Join(fieldDetails, ", ")))

			// Add example structure for nested objects to improve LLM compliance
			example := buildExampleJSON(schema)
			if example != "" {
				queryParts = append(queryParts, fmt.Sprintf("Example format: {\"%s\": %s}", fieldName, example))
			}
		} else {
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with the key '%s' containing the extracted object.", fieldName))
		}

	case "array":
		if schema.Items != nil {
			if schema.Items.Type == "object" && schema.Items.Properties != nil {
				// Build detailed description of array item structure
				requiredFields := make([]string, 0)
				optionalFields := make([]string, 0)

				for propName, propSchema := range schema.Items.Properties {
					fieldDesc := fmt.Sprintf("'%s' (%s)", propName, propSchema.Type)
					if contains(schema.Items.Required, propName) {
						requiredFields = append(requiredFields, fieldDesc)
					} else {
						optionalFields = append(optionalFields, fieldDesc)
					}
				}

				var itemDesc []string
				if len(requiredFields) > 0 {
					itemDesc = append(itemDesc, fmt.Sprintf("REQUIRED fields: %s", strings.Join(requiredFields, ", ")))
				}
				if len(optionalFields) > 0 {
					itemDesc = append(itemDesc, fmt.Sprintf("Optional fields: %s", strings.Join(optionalFields, ", ")))
				}

				queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with the key '%s' containing an array where each item is an object with %s.", fieldName, strings.Join(itemDesc, ". ")))
			} else {
				queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with the key '%s' containing an array of %s values.", fieldName, schema.Items.Type))
			}
		} else {
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with the key '%s' containing an array.", fieldName))
		}

	case "string":
		if len(schema.Enum) > 0 {
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object like {\"%s\": \"value\"} where value is EXACTLY one of: %s.", fieldName, strings.Join(schema.Enum, ", ")))
		} else {
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object like {\"%s\": \"extracted text\"}.", fieldName))
		}

	case "number", "integer":
		queryParts = append(queryParts, fmt.Sprintf("Return a JSON object like {\"%s\": <number>}.", fieldName))

	case "boolean":
		queryParts = append(queryParts, fmt.Sprintf("Return a JSON object like {\"%s\": true/false}.", fieldName))
	}

	return strings.Join(queryParts, " ")
}

// parseAndValidateJSON extracts JSON from response and validates against schema
func parseAndValidateJSON(result string, schema *JSONSchema) (map[string]interface{}, error) {
	// Remove markdown code blocks if present
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "```") {
		// Extract content between ``` markers
		lines := strings.Split(result, "\n")
		if len(lines) > 2 {
			// Remove first line (```json or ```) and last line (```)
			result = strings.Join(lines[1:len(lines)-1], "\n")
			result = strings.TrimSpace(result)
		}
	}

	// For non-object schemas (arrays, primitives), handle special cases
	if schema.Type != "object" {
		// Try parsing as direct value first
		var value interface{}
		parseErr := json.Unmarshal([]byte(result), &value)
		if parseErr == nil {
			// Check if it's a map (LLM wrapped the value in an object)
			if valueMap, ok := value.(map[string]interface{}); ok {
				// Try to unwrap based on expected type
				switch schema.Type {
				case "array":
					// Look for any array value in the map
					for _, v := range valueMap {
						if arr, ok := v.([]interface{}); ok {
							value = arr
							break
						}
					}
				case "string":
					// Look for any string value in the map
					for _, v := range valueMap {
						if str, ok := v.(string); ok {
							value = str
							break
						}
					}
				case "number", "integer":
					// Look for any number value in the map
					for _, v := range valueMap {
						switch v.(type) {
						case float64, float32, int, int32, int64:
							value = v
						}
					}
				case "boolean":
					// Look for any boolean value in the map
					for _, v := range valueMap {
						if b, ok := v.(bool); ok {
							value = b
							break
						}
					}
				default:
					// For other types, extract single-key value
					if len(valueMap) == 1 {
						for _, v := range valueMap {
							value = v
						}
					}
				}
			}

			// Validate the unwrapped value
			if err := validateValue(value, schema); err != nil {
				return nil, err
			}

			// Wrap in a map with a temp key
			return map[string]interface{}{"__value__": value}, nil
		}

		return nil, fmt.Errorf("failed to parse JSON: %v", parseErr)
	}

	// Try to find the outermost JSON object
	var parsed map[string]interface{}

	// First, try to parse the entire trimmed string
	if err := json.Unmarshal([]byte(result), &parsed); err == nil {
		if err := validateAgainstSchema(parsed, schema); err != nil {
			return nil, err
		}
		return parsed, nil
	}

	// If full-string parse failed, use balanced-brace extraction to find JSON objects
	jsonCandidates := extractBalancedJSON(result)

	if len(jsonCandidates) == 0 {
		return nil, fmt.Errorf("no JSON object found in response: %s", truncateForError(result))
	}

	// Try each candidate until we find one that validates
	for _, candidate := range jsonCandidates {
		var candidateMap map[string]interface{}
		if err := json.Unmarshal([]byte(candidate), &candidateMap); err == nil {
			if err := validateAgainstSchema(candidateMap, schema); err == nil {
				return candidateMap, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid JSON object matching schema found in response")
}

// extractBalancedJSON finds all top-level JSON objects in a string by tracking
// balanced braces. This handles arbitrary nesting depth, unlike the previous
// regex approach that could only match 1 level of nesting.
func extractBalancedJSON(s string) []string {
	var results []string
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '{' && !inString {
			// Found start of a potential JSON object; track balanced braces
			depth := 0
			inStr := false
			esc := false
			j := i

			for j < len(s) {
				ch := s[j]

				if esc {
					esc = false
					j++
					continue
				}

				if ch == '\\' && inStr {
					esc = true
					j++
					continue
				}

				if ch == '"' {
					inStr = !inStr
				}

				if !inStr {
					if ch == '{' {
						depth++
					} else if ch == '}' {
						depth--
						if depth == 0 {
							candidate := s[i : j+1]
							results = append(results, candidate)
							i = j // outer loop will increment past this
							break
						}
					}
				}

				j++
			}
		}

		// Track string state in the outer scan (for skipping { inside strings)
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
		}
	}

	return results
}

// truncateForError truncates a string for use in error messages
func truncateForError(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// validateAgainstSchema validates data against a JSON schema
func validateAgainstSchema(data map[string]interface{}, schema *JSONSchema) error {
	if schema.Type != "object" {
		return nil // Only validate object types for now
	}

	// Check required fields
	for _, required := range schema.Required {
		if _, exists := data[required]; !exists {
			return fmt.Errorf("missing required field: %s", required)
		}
	}

	// Validate properties
	if schema.Properties != nil {
		for key, fieldSchema := range schema.Properties {
			value, exists := data[key]
			if !exists && contains(schema.Required, key) {
				return fmt.Errorf("missing required field: %s", key)
			}
			if exists {
				if err := validateValue(value, fieldSchema); err != nil {
					return fmt.Errorf("field %s: %w", key, err)
				}
			}
		}
	}

	return nil
}

// validateValue validates a value against a schema
func validateValue(value interface{}, schema *JSONSchema) error {
	if value == nil && schema.Nullable {
		return nil
	}

	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "number", "integer":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			return nil
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		arr, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
		if schema.Items != nil {
			for i, item := range arr {
				if err := validateValue(item, schema.Items); err != nil {
					return fmt.Errorf("array item %d: %w", i, err)
				}
			}
		}
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
		return validateAgainstSchema(obj, schema)
	}

	return nil
}

func contains(arr []string, item string) bool {
	for _, v := range arr {
		if v == item {
			return true
		}
	}
	return false
}

// buildExampleJSON creates an example JSON structure for nested objects
func buildExampleJSON(schema *JSONSchema) string {
	if schema.Type != "object" || schema.Properties == nil {
		return ""
	}

	// Only generate examples for objects with required fields
	if len(schema.Required) == 0 {
		return ""
	}

	example := make(map[string]interface{})

	for fieldName, fieldSchema := range schema.Properties {
		isRequired := contains(schema.Required, fieldName)

		// Only include required fields in example
		if !isRequired {
			continue
		}

		switch fieldSchema.Type {
		case "string":
			if len(fieldSchema.Enum) > 0 {
				example[fieldName] = fieldSchema.Enum[0]
			} else {
				example[fieldName] = "example value"
			}
		case "number":
			// Use sensible defaults for common field names
			if strings.Contains(strings.ToLower(fieldName), "score") || strings.Contains(strings.ToLower(fieldName), "sentiment") {
				example[fieldName] = 3
			} else if strings.Contains(strings.ToLower(fieldName), "confidence") {
				example[fieldName] = 0.8
			} else {
				example[fieldName] = 0
			}
		case "integer":
			if strings.Contains(strings.ToLower(fieldName), "score") || strings.Contains(strings.ToLower(fieldName), "sentiment") {
				example[fieldName] = 3
			} else {
				example[fieldName] = 0
			}
		case "boolean":
			example[fieldName] = true
		case "array":
			example[fieldName] = []interface{}{}
		case "object":
			// Recursively build nested object
			nestedExample := buildExampleJSON(fieldSchema)
			if nestedExample != "" {
				var nested map[string]interface{}
				if err := json.Unmarshal([]byte(nestedExample), &nested); err == nil {
					example[fieldName] = nested
				}
			} else {
				example[fieldName] = map[string]interface{}{}
			}
		}
	}

	if len(example) == 0 {
		return ""
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(example)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

// buildValidationFeedback creates detailed feedback for LLM retry attempts
func buildValidationFeedback(validationErr error, schema *JSONSchema, previousResponse string) string {
	errMsg := validationErr.Error()

	var feedback strings.Builder
	feedback.WriteString("VALIDATION ERROR - Your previous response was invalid.\n\n")
	feedback.WriteString(fmt.Sprintf("ERROR: %s\n\n", errMsg))

	// Extract what field caused the issue
	if strings.Contains(errMsg, "missing required field:") {
		// Parse out the field name
		fieldName := strings.TrimPrefix(errMsg, "missing required field: ")
		fieldName = strings.TrimSpace(fieldName)

		feedback.WriteString("SPECIFIC ISSUE:\n")
		feedback.WriteString(fmt.Sprintf("The field '%s' is REQUIRED but was not provided.\n\n", fieldName))

		// Find the schema for this field and provide details
		if schema.Type == "object" && schema.Properties != nil {
			if fieldSchema, exists := schema.Properties[fieldName]; exists {
				feedback.WriteString("FIELD REQUIREMENTS:\n")
				feedback.WriteString(fmt.Sprintf("- Field name: '%s'\n", fieldName))
				feedback.WriteString(fmt.Sprintf("- Type: %s\n", fieldSchema.Type))

				if fieldSchema.Type == "object" && len(fieldSchema.Required) > 0 {
					feedback.WriteString(fmt.Sprintf("- This is an object with required fields: %s\n", strings.Join(fieldSchema.Required, ", ")))

					if fieldSchema.Properties != nil {
						feedback.WriteString("\nNESTED FIELD DETAILS:\n")
						for nestedField, nestedSchema := range fieldSchema.Properties {
							isRequired := contains(fieldSchema.Required, nestedField)
							requiredMark := ""
							if isRequired {
								requiredMark = " [REQUIRED]"
							}
							feedback.WriteString(fmt.Sprintf("  - %s: %s%s\n", nestedField, nestedSchema.Type, requiredMark))
						}
					}
				}

				if fieldSchema.Type == "array" && fieldSchema.Items != nil {
					feedback.WriteString(fmt.Sprintf("- This is an array of: %s\n", fieldSchema.Items.Type))
				}
			}
		}
	} else if strings.Contains(errMsg, "expected") {
		feedback.WriteString("SPECIFIC ISSUE:\n")
		feedback.WriteString("Type mismatch - you provided the wrong data type.\n\n")
	}

	// Show a snippet of what they provided
	if len(previousResponse) > 0 {
		snippet := previousResponse
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		feedback.WriteString("\nYOUR PREVIOUS RESPONSE:\n")
		feedback.WriteString(snippet)
		feedback.WriteString("\n\n")
	}

	// Provide the full expected schema
	schemaJSON, _ := json.Marshal(schema)
	feedback.WriteString("EXPECTED SCHEMA:\n")
	feedback.WriteString(string(schemaJSON))
	feedback.WriteString("\n\n")

	feedback.WriteString("ACTION REQUIRED:\n")
	feedback.WriteString("Please provide a COMPLETE and VALID JSON response that includes ALL required fields.\n")
	feedback.WriteString("Remember:\n")
	feedback.WriteString("1. Include ALL required fields (see above)\n")
	feedback.WriteString("2. Use correct data types (string, number, object, array)\n")
	feedback.WriteString("3. For nested objects, include ALL their required fields too\n")
	feedback.WriteString("4. Return ONLY valid JSON - no markdown, no explanations\n")

	return feedback.String()
}
