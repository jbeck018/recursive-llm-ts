package rlm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// StructuredCompletion executes a structured completion with schema validation
func (r *RLM) StructuredCompletion(query string, context string, config *StructuredConfig) (map[string]interface{}, RLMStats, error) {
	if config == nil || config.Schema == nil {
		return nil, RLMStats{}, fmt.Errorf("structured config and schema are required")
	}

	// Set defaults
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	// Decompose schema into sub-tasks
	subTasks := decomposeSchema(config.Schema)

	// If simple schema or parallel disabled, use direct method
	if len(subTasks) <= 2 || !config.ParallelExecution {
		return r.structuredCompletionDirect(query, context, config)
	}

	// Execute with parallel goroutines
	return r.structuredCompletionParallel(query, context, config, subTasks)
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
	
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		// Call LLM directly without REPL
		messages := []Message{
			{Role: "system", Content: "You are a data extraction assistant. Respond only with valid JSON objects."},
			{Role: "user", Content: prompt},
		}
		
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
				
				// Retry with detailed error feedback
				messages = []Message{
					{Role: "system", Content: "You are a data extraction assistant. Respond only with valid JSON objects."},
					{Role: "user", Content: prompt},
					{Role: "assistant", Content: result},
					{Role: "user", Content: validationFeedback},
				}
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
	errChan := make(chan error, len(subTasks))
	
	totalStats := RLMStats{}
	var statsMutex sync.Mutex

	for _, task := range subTasks {
		wg.Add(1)
		go func(t SubTask) {
			defer wg.Done()

			taskQuery := fmt.Sprintf("%s\n\nSpecific focus: %s", query, t.Query)
			taskConfig := &StructuredConfig{
				Schema:            t.Schema,
				ParallelExecution: false, // Disable nested parallelization
				MaxRetries:        config.MaxRetries,
			}

			result, stats, err := r.structuredCompletionDirect(taskQuery, context, taskConfig)
			if err != nil {
				errChan <- fmt.Errorf("task %s failed: %w", t.ID, err)
				return
			}

			resultsMutex.Lock()
			fieldName := strings.TrimPrefix(t.ID, "field_")
			// If result has the __value__ wrapper (non-object type), unwrap it
			if val, ok := result["__value__"]; ok {
				results[fieldName] = val
			} else {
				results[fieldName] = result
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
		}(task)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, totalStats, <-errChan
	}

	// Validate merged result against full schema
	if err := validateAgainstSchema(results, config.Schema); err != nil {
		return nil, totalStats, fmt.Errorf("merged result validation failed: %w", err)
	}

	return results, totalStats, nil
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
			if fieldSchema.Type == "number" {
				if strings.Contains(strings.ToLower(fieldName), "sentiment") || strings.Contains(strings.ToLower(fieldName), "score") {
					constraints = append(constraints, fmt.Sprintf("- %s must be a number between 1 and 5 (inclusive)", fieldName))
				}
			}
			if fieldSchema.Enum != nil && len(fieldSchema.Enum) > 0 {
				constraints = append(constraints, fmt.Sprintf("- %s must be EXACTLY one of these values: %s (use these exact strings, do not modify)", fieldName, strings.Join(fieldSchema.Enum, ", ")))
			}
			if fieldSchema.Type == "array" {
				constraints = append(constraints, fmt.Sprintf("- %s must be a JSON array []", fieldName))
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
				if fieldSchema.Type == "number" && strings.Contains(strings.ToLower(fieldName), "sentiment") {
					constraints = append(constraints, fmt.Sprintf("- Each item's %s must be between 1 and 5", fieldName))
				}
				if fieldSchema.Enum != nil && len(fieldSchema.Enum) > 0 {
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
	queryParts = append(queryParts, fmt.Sprintf("Extract the %s from the conversation.", fieldName))
	
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
			queryParts = append(queryParts, fmt.Sprintf("Return a JSON object with these REQUIRED fields: %s.", strings.Join(fieldDetails, ", ")))
		} else {
			queryParts = append(queryParts, "Return a JSON object.")
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
				
				queryParts = append(queryParts, fmt.Sprintf("Return a JSON array where each item is an object with %s.", strings.Join(itemDesc, ". ")))
			} else {
				queryParts = append(queryParts, fmt.Sprintf("Return a JSON array of %s values.", schema.Items.Type))
			}
		} else {
			queryParts = append(queryParts, "Return a JSON array.")
		}
		
	case "string":
		if schema.Enum != nil && len(schema.Enum) > 0 {
			queryParts = append(queryParts, fmt.Sprintf("Return EXACTLY one of these values: %s (use exact strings).", strings.Join(schema.Enum, ", ")))
		} else {
			queryParts = append(queryParts, "Return a string value.")
		}
		
	case "number":
		queryParts = append(queryParts, "Return a numeric value.")
		
	case "boolean":
		queryParts = append(queryParts, "Return a boolean value (true or false).")
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
				case "number":
					// Look for any number value in the map
					for _, v := range valueMap {
						switch v.(type) {
						case float64, float32, int, int32, int64:
							value = v
							break
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
							break
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
	
	// If that fails, try to extract JSON with regex
	re := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := re.FindAllString(result, -1)
	
	if len(matches) == 0 {
		return nil, fmt.Errorf("no JSON object found in response: %s", result)
	}
	
	// Try each match until we find one that validates
	for _, match := range matches {
		var candidate map[string]interface{}
		if err := json.Unmarshal([]byte(match), &candidate); err == nil {
			if err := validateAgainstSchema(candidate, schema); err == nil {
				return candidate, nil
			}
		}
	}
	
	return nil, fmt.Errorf("no valid JSON object matching schema found in response")
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
	case "number":
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

// buildValidationFeedback creates detailed feedback for LLM retry attempts
func buildValidationFeedback(validationErr error, schema *JSONSchema, previousResponse string) string {
	errMsg := validationErr.Error()
	
	var feedback strings.Builder
	feedback.WriteString("❌ VALIDATION ERROR - Your previous response was invalid.\n\n")
	feedback.WriteString(fmt.Sprintf("ERROR: %s\n\n", errMsg))
	
	// Extract what field caused the issue
	if strings.Contains(errMsg, "missing required field:") {
		// Parse out the field name
		fieldName := strings.TrimPrefix(errMsg, "missing required field: ")
		fieldName = strings.TrimSpace(fieldName)
		
		feedback.WriteString("🔍 SPECIFIC ISSUE:\n")
		feedback.WriteString(fmt.Sprintf("The field '%s' is REQUIRED but was not provided.\n\n", fieldName))
		
		// Find the schema for this field and provide details
		if schema.Type == "object" && schema.Properties != nil {
			if fieldSchema, exists := schema.Properties[fieldName]; exists {
				feedback.WriteString("📋 FIELD REQUIREMENTS:\n")
				feedback.WriteString(fmt.Sprintf("- Field name: '%s'\n", fieldName))
				feedback.WriteString(fmt.Sprintf("- Type: %s\n", fieldSchema.Type))
				
				if fieldSchema.Type == "object" && len(fieldSchema.Required) > 0 {
					feedback.WriteString(fmt.Sprintf("- This is an object with required fields: %s\n", strings.Join(fieldSchema.Required, ", ")))
					
					if fieldSchema.Properties != nil {
						feedback.WriteString("\n📝 NESTED FIELD DETAILS:\n")
						for nestedField, nestedSchema := range fieldSchema.Properties {
							isRequired := contains(fieldSchema.Required, nestedField)
							requiredMark := ""
							if isRequired {
								requiredMark = " [REQUIRED]"
							}
							feedback.WriteString(fmt.Sprintf("  • %s: %s%s\n", nestedField, nestedSchema.Type, requiredMark))
						}
					}
				}
				
				if fieldSchema.Type == "array" && fieldSchema.Items != nil {
					feedback.WriteString(fmt.Sprintf("- This is an array of: %s\n", fieldSchema.Items.Type))
				}
			}
		}
	} else if strings.Contains(errMsg, "expected") {
		feedback.WriteString("🔍 SPECIFIC ISSUE:\n")
		feedback.WriteString("Type mismatch - you provided the wrong data type.\n\n")
	}
	
	// Show a snippet of what they provided
	if len(previousResponse) > 0 {
		snippet := previousResponse
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		feedback.WriteString("\n📤 YOUR PREVIOUS RESPONSE:\n")
		feedback.WriteString(snippet)
		feedback.WriteString("\n\n")
	}
	
	feedback.WriteString("✅ ACTION REQUIRED:\n")
	feedback.WriteString("Please provide a COMPLETE and VALID JSON response that includes ALL required fields.\n")
	feedback.WriteString("Remember:\n")
	feedback.WriteString("1. Include ALL required fields (see above)\n")
	feedback.WriteString("2. Use correct data types (string, number, object, array)\n")
	feedback.WriteString("3. For nested objects, include ALL their required fields too\n")
	feedback.WriteString("4. Return ONLY valid JSON - no markdown, no explanations\n")
	
	return feedback.String()
}
