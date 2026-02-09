package rlm

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ─── extractBalancedJSON Tests ───────────────────────────────────────────────

func TestExtractBalancedJSON_SimpleObject(t *testing.T) {
	input := `{"name": "Alice", "age": 30}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != input {
		t.Errorf("expected %q, got %q", input, results[0])
	}
}

func TestExtractBalancedJSON_NestedObject(t *testing.T) {
	input := `{"user": {"name": "Alice", "address": {"city": "NYC", "zip": "10001"}}}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(results[0]), &parsed); err != nil {
		t.Fatalf("failed to parse extracted JSON: %v", err)
	}

	user, ok := parsed["user"].(map[string]interface{})
	if !ok {
		t.Fatal("missing or invalid 'user' field")
	}
	address, ok := user["address"].(map[string]interface{})
	if !ok {
		t.Fatal("missing or invalid 'address' field")
	}
	if address["city"] != "NYC" {
		t.Errorf("expected city 'NYC', got %v", address["city"])
	}
}

func TestExtractBalancedJSON_DeeplyNested(t *testing.T) {
	// 4 levels of nesting - the old regex would fail on this
	input := `{"a": {"b": {"c": {"d": "deep"}}}}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(results[0]), &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	// Navigate to the deepest level
	a := parsed["a"].(map[string]interface{})
	b := a["b"].(map[string]interface{})
	c := b["c"].(map[string]interface{})
	if c["d"] != "deep" {
		t.Errorf("expected 'deep', got %v", c["d"])
	}
}

func TestExtractBalancedJSON_WithSurroundingText(t *testing.T) {
	input := `Here is the JSON result: {"key": "value"} and some trailing text.`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != `{"key": "value"}` {
		t.Errorf("expected clean JSON, got %q", results[0])
	}
}

func TestExtractBalancedJSON_MultipleObjects(t *testing.T) {
	input := `First: {"a": 1} Second: {"b": 2}`
	results := extractBalancedJSON(input)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestExtractBalancedJSON_BracesInStrings(t *testing.T) {
	input := `{"text": "This has { braces } inside", "value": 42}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(results[0]), &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if parsed["text"] != "This has { braces } inside" {
		t.Errorf("string content not preserved: %v", parsed["text"])
	}
}

func TestExtractBalancedJSON_EscapedQuotes(t *testing.T) {
	input := `{"text": "He said \"hello\"", "count": 1}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(results[0]), &parsed); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
}

func TestExtractBalancedJSON_NoObjects(t *testing.T) {
	input := `Just some plain text with no JSON`
	results := extractBalancedJSON(input)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestExtractBalancedJSON_ComplexNestedWithArrays(t *testing.T) {
	// This mimics the sentiment analysis schema structure
	input := `{"sentimentValue": 3, "sentimentExplanation": "Positive", "phrases": [{"sentimentValue": 4, "phrase": "Great work"}, {"sentimentValue": 2, "phrase": "Could improve"}], "keyMoments": [{"phrase": "Budget discussion", "type": "budget_concern"}]}`
	results := extractBalancedJSON(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(results[0]), &parsed); err != nil {
		t.Fatalf("failed to parse complex nested JSON: %v", err)
	}

	phrases, ok := parsed["phrases"].([]interface{})
	if !ok {
		t.Fatal("expected 'phrases' to be an array")
	}
	if len(phrases) != 2 {
		t.Errorf("expected 2 phrases, got %d", len(phrases))
	}
}

// ─── wrapFieldSchema Tests ──────────────────────────────────────────────────

func TestWrapFieldSchema_Number(t *testing.T) {
	min := 1.0
	max := 5.0
	fieldSchema := &JSONSchema{
		Type:    "number",
		Minimum: &min,
		Maximum: &max,
	}

	wrapped := wrapFieldSchema("sentimentValue", fieldSchema)

	if wrapped.Type != "object" {
		t.Errorf("expected object type, got %s", wrapped.Type)
	}
	if len(wrapped.Required) != 1 || wrapped.Required[0] != "sentimentValue" {
		t.Errorf("expected required [sentimentValue], got %v", wrapped.Required)
	}
	if wrapped.Properties["sentimentValue"] != fieldSchema {
		t.Error("inner schema should reference the original")
	}
}

func TestWrapFieldSchema_Array(t *testing.T) {
	fieldSchema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"phrase": {Type: "string"},
				"score":  {Type: "number"},
			},
			Required: []string{"phrase", "score"},
		},
	}

	wrapped := wrapFieldSchema("phrases", fieldSchema)

	if wrapped.Type != "object" {
		t.Errorf("expected object type, got %s", wrapped.Type)
	}
	innerSchema := wrapped.Properties["phrases"]
	if innerSchema.Type != "array" {
		t.Errorf("expected inner type array, got %s", innerSchema.Type)
	}
}

func TestWrapFieldSchema_String(t *testing.T) {
	fieldSchema := &JSONSchema{
		Type: "string",
	}

	wrapped := wrapFieldSchema("explanation", fieldSchema)

	if wrapped.Type != "object" {
		t.Errorf("expected object type, got %s", wrapped.Type)
	}
	if wrapped.Properties["explanation"].Type != "string" {
		t.Error("inner schema should be string type")
	}
}

// ─── parseAndValidateJSON Tests ─────────────────────────────────────────────

func TestParseAndValidateJSON_SimpleObject(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name", "age"},
	}

	result, err := parseAndValidateJSON(`{"name": "Alice", "age": 30}`, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got %v", result["name"])
	}
}

func TestParseAndValidateJSON_WithMarkdownCodeBlock(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"key": {Type: "string"},
		},
		Required: []string{"key"},
	}

	input := "```json\n{\"key\": \"value\"}\n```"
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected 'value', got %v", result["key"])
	}
}

func TestParseAndValidateJSON_WithSurroundingText(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"status": {Type: "string"},
		},
		Required: []string{"status"},
	}

	input := `Here is the result: {"status": "ok"} hope that helps!`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected 'ok', got %v", result["status"])
	}
}

func TestParseAndValidateJSON_DeeplyNestedObject(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"data": {
				Type: "object",
				Properties: map[string]*JSONSchema{
					"inner": {
						Type: "object",
						Properties: map[string]*JSONSchema{
							"value": {Type: "string"},
						},
						Required: []string{"value"},
					},
				},
				Required: []string{"inner"},
			},
		},
		Required: []string{"data"},
	}

	input := `{"data": {"inner": {"value": "deep"}}}`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := result["data"].(map[string]interface{})
	inner := data["inner"].(map[string]interface{})
	if inner["value"] != "deep" {
		t.Errorf("expected 'deep', got %v", inner["value"])
	}
}

func TestParseAndValidateJSON_NonObjectSchema_Array(t *testing.T) {
	schema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "string",
		},
	}

	input := `["a", "b", "c"]`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := result["__value__"].([]interface{})
	if !ok {
		t.Fatalf("expected array in __value__, got %T", result["__value__"])
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 items, got %d", len(arr))
	}
}

func TestParseAndValidateJSON_NonObjectSchema_WrappedArray(t *testing.T) {
	schema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "string",
		},
	}

	// LLM sometimes wraps array in an object
	input := `{"items": ["a", "b", "c"]}`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := result["__value__"].([]interface{})
	if !ok {
		t.Fatalf("expected array in __value__, got %T", result["__value__"])
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 items, got %d", len(arr))
	}
}

func TestParseAndValidateJSON_NonObjectSchema_Number(t *testing.T) {
	schema := &JSONSchema{
		Type: "number",
	}

	input := `42`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := result["__value__"].(float64)
	if !ok {
		t.Fatalf("expected float64 in __value__, got %T", result["__value__"])
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestParseAndValidateJSON_NonObjectSchema_WrappedNumber(t *testing.T) {
	schema := &JSONSchema{
		Type: "number",
	}

	// LLM wraps number in an object
	input := `{"sentimentValue": 3.5}`
	result, err := parseAndValidateJSON(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := result["__value__"].(float64)
	if !ok {
		t.Fatalf("expected float64 in __value__, got %T", result["__value__"])
	}
	if val != 3.5 {
		t.Errorf("expected 3.5, got %v", val)
	}
}

func TestParseAndValidateJSON_MissingRequiredField(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name", "age"},
	}

	input := `{"name": "Alice"}`
	_, err := parseAndValidateJSON(input, schema)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "missing required field: age") {
		t.Errorf("expected missing field error, got: %v", err)
	}
}

func TestParseAndValidateJSON_TypeMismatch(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"count": {Type: "number"},
		},
		Required: []string{"count"},
	}

	input := `{"count": "not a number"}`
	_, err := parseAndValidateJSON(input, schema)
	if err == nil {
		t.Fatal("expected type validation error")
	}
}

// ─── validateValue Tests ────────────────────────────────────────────────────

func TestValidateValue_IntegerAlsoAcceptsFloat64(t *testing.T) {
	schema := &JSONSchema{Type: "integer"}

	// JSON numbers always decode as float64, but integer type should accept them
	err := validateValue(float64(42), schema)
	if err != nil {
		t.Errorf("integer schema should accept float64: %v", err)
	}
}

func TestValidateValue_NumberType(t *testing.T) {
	schema := &JSONSchema{Type: "number"}

	if err := validateValue(float64(3.14), schema); err != nil {
		t.Errorf("should accept float: %v", err)
	}
	if err := validateValue("not a number", schema); err == nil {
		t.Error("should reject string")
	}
}

func TestValidateValue_NullableField(t *testing.T) {
	schema := &JSONSchema{Type: "string", Nullable: true}

	if err := validateValue(nil, schema); err != nil {
		t.Errorf("nullable field should accept nil: %v", err)
	}
}

func TestValidateValue_ArrayWithItems(t *testing.T) {
	schema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"phrase": {Type: "string"},
				"score":  {Type: "number"},
			},
			Required: []string{"phrase", "score"},
		},
	}

	valid := []interface{}{
		map[string]interface{}{"phrase": "good", "score": float64(4)},
		map[string]interface{}{"phrase": "bad", "score": float64(1)},
	}

	if err := validateValue(valid, schema); err != nil {
		t.Errorf("valid array should pass: %v", err)
	}

	invalid := []interface{}{
		map[string]interface{}{"phrase": "missing score"},
	}

	if err := validateValue(invalid, schema); err == nil {
		t.Error("invalid array item should fail")
	}
}

// ─── decomposeSchema Tests ──────────────────────────────────────────────────

func TestDecomposeSchema_SentimentAnalysis(t *testing.T) {
	min := 1.0
	max := 5.0

	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"sentimentValue": {
				Type:    "number",
				Minimum: &min,
				Maximum: &max,
			},
			"sentimentExplanation": {
				Type: "string",
			},
			"phrases": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"sentimentValue": {Type: "number", Minimum: &min, Maximum: &max},
						"phrase":         {Type: "string"},
					},
					Required: []string{"sentimentValue", "phrase"},
				},
			},
			"keyMoments": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"phrase": {Type: "string"},
						"type":  {Type: "string", Enum: []string{"churn_mention", "personnel_change"}},
					},
					Required: []string{"phrase", "type"},
				},
			},
		},
		Required: []string{"sentimentValue", "sentimentExplanation", "phrases", "keyMoments"},
	}

	subTasks := decomposeSchema(schema)

	if len(subTasks) != 4 {
		t.Fatalf("expected 4 subtasks, got %d", len(subTasks))
	}

	// Check that each field has a corresponding subtask
	fieldNames := make(map[string]bool)
	for _, task := range subTasks {
		fieldName := strings.TrimPrefix(task.ID, "field_")
		fieldNames[fieldName] = true
	}

	for _, expected := range []string{"sentimentValue", "sentimentExplanation", "phrases", "keyMoments"} {
		if !fieldNames[expected] {
			t.Errorf("missing subtask for field: %s", expected)
		}
	}
}

func TestDecomposeSchema_NonObject(t *testing.T) {
	schema := &JSONSchema{Type: "array", Items: &JSONSchema{Type: "string"}}
	subTasks := decomposeSchema(schema)
	if len(subTasks) != 0 {
		t.Errorf("non-object schema should produce 0 subtasks, got %d", len(subTasks))
	}
}

// ─── generateFieldQuery Tests ───────────────────────────────────────────────

func TestGenerateFieldQuery_Number(t *testing.T) {
	schema := &JSONSchema{Type: "number"}
	query := generateFieldQuery("sentimentValue", schema)

	if !strings.Contains(query, "sentimentValue") {
		t.Error("query should reference the field name")
	}
	if !strings.Contains(query, `{"sentimentValue": <number>}`) {
		t.Errorf("query should include JSON object format example, got: %s", query)
	}
}

func TestGenerateFieldQuery_StringWithEnum(t *testing.T) {
	schema := &JSONSchema{
		Type: "string",
		Enum: []string{"positive", "negative", "neutral"},
	}
	query := generateFieldQuery("sentiment", schema)

	if !strings.Contains(query, "positive") {
		t.Error("query should list enum values")
	}
	if !strings.Contains(query, "EXACTLY one of") {
		t.Error("query should mention exact match")
	}
}

func TestGenerateFieldQuery_ArrayWithObjectItems(t *testing.T) {
	schema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name":  {Type: "string"},
				"score": {Type: "number"},
			},
			Required: []string{"name", "score"},
		},
	}
	query := generateFieldQuery("results", schema)

	if !strings.Contains(query, "results") {
		t.Error("query should reference field name")
	}
	if !strings.Contains(query, "REQUIRED") {
		t.Error("query should mention required fields")
	}
}

// ─── generateSchemaConstraints Tests ────────────────────────────────────────

func TestGenerateSchemaConstraints_WithNumberRange(t *testing.T) {
	min := 1.0
	max := 5.0
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"score": {Type: "number", Minimum: &min, Maximum: &max},
		},
	}

	constraints := generateSchemaConstraints(schema)
	if !strings.Contains(constraints, ">= 1") {
		t.Error("should include minimum constraint")
	}
	if !strings.Contains(constraints, "<= 5") {
		t.Error("should include maximum constraint")
	}
}

func TestGenerateSchemaConstraints_WithEnum(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"status": {Type: "string", Enum: []string{"active", "inactive"}},
		},
	}

	constraints := generateSchemaConstraints(schema)
	if !strings.Contains(constraints, "EXACTLY") {
		t.Error("should emphasize exact match")
	}
	if !strings.Contains(constraints, "active") {
		t.Error("should list enum values")
	}
}

// ─── buildValidationFeedback Tests ──────────────────────────────────────────

func TestBuildValidationFeedback_MissingField(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name":  {Type: "string"},
			"email": {Type: "string"},
		},
		Required: []string{"name", "email"},
	}

	err := fmt.Errorf("missing required field: email")
	feedback := buildValidationFeedback(err, schema, `{"name": "Alice"}`)

	if !strings.Contains(feedback, "email") {
		t.Error("feedback should mention the missing field")
	}
	if !strings.Contains(feedback, "REQUIRED") {
		t.Error("feedback should indicate field is required")
	}
	if !strings.Contains(feedback, "EXPECTED SCHEMA") {
		t.Error("feedback should include the expected schema")
	}
}

func TestBuildValidationFeedback_TypeMismatch(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"count": {Type: "number"},
		},
		Required: []string{"count"},
	}

	err := fmt.Errorf("field count: expected number, got string")
	feedback := buildValidationFeedback(err, schema, `{"count": "five"}`)

	if !strings.Contains(feedback, "Type mismatch") {
		t.Error("feedback should mention type mismatch")
	}
}

// ─── buildExampleJSON Tests ─────────────────────────────────────────────────

func TestBuildExampleJSON_SimpleObject(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name":  {Type: "string"},
			"score": {Type: "number"},
		},
		Required: []string{"name", "score"},
	}

	example := buildExampleJSON(schema)
	if example == "" {
		t.Fatal("expected non-empty example")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(example), &parsed); err != nil {
		t.Fatalf("example should be valid JSON: %v", err)
	}
	if _, ok := parsed["name"].(string); !ok {
		t.Error("example should have string 'name'")
	}
}

func TestBuildExampleJSON_WithEnum(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"status": {Type: "string", Enum: []string{"active", "inactive"}},
		},
		Required: []string{"status"},
	}

	example := buildExampleJSON(schema)
	if !strings.Contains(example, "active") {
		t.Error("example should use first enum value")
	}
}

func TestBuildExampleJSON_NoRequiredFields(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"optional": {Type: "string"},
		},
	}

	example := buildExampleJSON(schema)
	if example != "" {
		t.Error("schema with no required fields should produce empty example")
	}
}

// ─── Integration-style tests for the full parallel flow ─────────────────────

func TestParallelResultMerging_SimulatedWorkflow(t *testing.T) {
	// Simulate what happens when parallel results are merged
	min := 1.0
	max := 5.0

	fullSchema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"sentimentValue": {Type: "number", Minimum: &min, Maximum: &max},
			"explanation":    {Type: "string"},
			"tags":           {Type: "array", Items: &JSONSchema{Type: "string"}},
		},
		Required: []string{"sentimentValue", "explanation", "tags"},
	}

	// Simulate parallel results after unwrapping from wrapped schemas
	results := map[string]interface{}{
		"sentimentValue": float64(4),
		"explanation":    "Very positive conversation",
		"tags":           []interface{}{"positive", "engaged"},
	}

	// Validate merged result against full schema
	err := validateAgainstSchema(results, fullSchema)
	if err != nil {
		t.Errorf("merged results should validate: %v", err)
	}
}

func TestParallelResultMerging_NestedObjectField(t *testing.T) {
	// Test that object-type fields survive the wrap/unwrap cycle
	fullSchema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"metadata": {
				Type: "object",
				Properties: map[string]*JSONSchema{
					"author": {Type: "string"},
					"date":   {Type: "string"},
				},
				Required: []string{"author", "date"},
			},
			"summary": {Type: "string"},
		},
		Required: []string{"metadata", "summary"},
	}

	// Simulate what we'd get from the wrapped sub-task for "metadata"
	wrappedResponse := `{"metadata": {"author": "Alice", "date": "2024-01-01"}}`
	wrappedSchema := wrapFieldSchema("metadata", fullSchema.Properties["metadata"])

	result, err := parseAndValidateJSON(wrappedResponse, wrappedSchema)
	if err != nil {
		t.Fatalf("wrapped response should parse: %v", err)
	}

	// Extract field value (simulating what structuredCompletionParallel does)
	metadataValue, ok := result["metadata"]
	if !ok {
		t.Fatal("expected 'metadata' key in result")
	}

	metadataObj, ok := metadataValue.(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be an object, got %T", metadataValue)
	}

	if metadataObj["author"] != "Alice" {
		t.Errorf("expected author 'Alice', got %v", metadataObj["author"])
	}
}

func TestParallelResultMerging_ArrayField(t *testing.T) {
	// Test that array-type fields survive the wrap/unwrap cycle
	arraySchema := &JSONSchema{
		Type: "array",
		Items: &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name":  {Type: "string"},
				"score": {Type: "number"},
			},
			Required: []string{"name", "score"},
		},
	}

	wrappedSchema := wrapFieldSchema("phrases", arraySchema)
	wrappedResponse := `{"phrases": [{"name": "hello", "score": 4.5}, {"name": "goodbye", "score": 2.0}]}`

	result, err := parseAndValidateJSON(wrappedResponse, wrappedSchema)
	if err != nil {
		t.Fatalf("wrapped array response should parse: %v", err)
	}

	phrasesValue, ok := result["phrases"]
	if !ok {
		t.Fatal("expected 'phrases' key in result")
	}

	phrases, ok := phrasesValue.([]interface{})
	if !ok {
		t.Fatalf("expected phrases to be an array, got %T", phrasesValue)
	}

	if len(phrases) != 2 {
		t.Errorf("expected 2 phrases, got %d", len(phrases))
	}
}

func TestParallelResultMerging_NumberField(t *testing.T) {
	// Test that number-type fields survive the wrap/unwrap cycle
	min := 1.0
	max := 5.0
	numberSchema := &JSONSchema{Type: "number", Minimum: &min, Maximum: &max}
	wrappedSchema := wrapFieldSchema("sentimentValue", numberSchema)

	wrappedResponse := `{"sentimentValue": 3.5}`
	result, err := parseAndValidateJSON(wrappedResponse, wrappedSchema)
	if err != nil {
		t.Fatalf("wrapped number response should parse: %v", err)
	}

	val, ok := result["sentimentValue"]
	if !ok {
		t.Fatal("expected 'sentimentValue' key")
	}
	if val != 3.5 {
		t.Errorf("expected 3.5, got %v", val)
	}
}

func TestParallelResultMerging_StringEnumField(t *testing.T) {
	enumSchema := &JSONSchema{
		Type: "string",
		Enum: []string{"positive", "negative", "neutral"},
	}
	wrappedSchema := wrapFieldSchema("sentiment", enumSchema)

	wrappedResponse := `{"sentiment": "positive"}`
	result, err := parseAndValidateJSON(wrappedResponse, wrappedSchema)
	if err != nil {
		t.Fatalf("wrapped enum response should parse: %v", err)
	}

	val, ok := result["sentiment"]
	if !ok {
		t.Fatal("expected 'sentiment' key")
	}
	if val != "positive" {
		t.Errorf("expected 'positive', got %v", val)
	}
}
