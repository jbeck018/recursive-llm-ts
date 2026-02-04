package rlm

import (
	"encoding/json"
	"testing"
)

func TestNewSchemaValidator(t *testing.T) {
	tests := []struct {
		name    string
		schema  *JSONSchema
		wantErr bool
	}{
		{
			name:    "nil schema",
			schema:  nil,
			wantErr: true,
		},
		{
			name: "simple string schema",
			schema: &JSONSchema{
				Type: "string",
			},
			wantErr: false,
		},
		{
			name: "object schema with properties",
			schema: &JSONSchema{
				Type: "object",
				Properties: map[string]*JSONSchema{
					"name": {Type: "string"},
					"age":  {Type: "number"},
				},
				Required: []string{"name"},
			},
			wantErr: false,
		},
		{
			name: "array schema",
			schema: &JSONSchema{
				Type: "array",
				Items: &JSONSchema{
					Type: "string",
				},
			},
			wantErr: false,
		},
		{
			name: "nested object schema",
			schema: &JSONSchema{
				Type: "object",
				Properties: map[string]*JSONSchema{
					"address": {
						Type: "object",
						Properties: map[string]*JSONSchema{
							"street": {Type: "string"},
							"city":   {Type: "string"},
						},
						Required: []string{"street", "city"},
					},
				},
				Required: []string{"address"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewSchemaValidator(tt.schema)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if validator == nil {
				t.Error("expected non-nil validator")
			}
		})
	}
}

func TestSchemaValidatorValidate(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name"},
	}

	validator, err := NewSchemaValidator(schema)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid data with all fields",
			data: map[string]interface{}{
				"name": "Alice",
				"age":  float64(30),
			},
			wantErr: false,
		},
		{
			name: "valid data with only required fields",
			data: map[string]interface{}{
				"name": "Bob",
			},
			wantErr: false,
		},
		{
			name: "invalid - missing required field",
			data: map[string]interface{}{
				"age": float64(25),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.data)
			if tt.wantErr && err == nil {
				t.Error("expected validation error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"status": {Type: "string"},
		},
		Required: []string{"status"},
	}

	validator, err := NewSchemaValidator(schema)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	validJSON := `{"status": "ok"}`
	if err := validator.ValidateJSON([]byte(validJSON)); err != nil {
		t.Errorf("expected valid JSON to pass: %v", err)
	}

	invalidJSON := `{"missing": "field"}`
	if err := validator.ValidateJSON([]byte(invalidJSON)); err == nil {
		t.Error("expected invalid JSON to fail validation")
	}

	malformedJSON := `{not valid json`
	if err := validator.ValidateJSON([]byte(malformedJSON)); err == nil {
		t.Error("expected malformed JSON to fail")
	}
}

func TestJSONSchemaConversion(t *testing.T) {
	original := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name":  {Type: "string"},
			"score": {Type: "number"},
		},
		Required: []string{"name", "score"},
	}

	// Convert to Google schema and back
	googleSchema, err := JSONSchemaToGoogleSchema(original)
	if err != nil {
		t.Fatalf("failed to convert to Google schema: %v", err)
	}
	if googleSchema == nil {
		t.Fatal("expected non-nil Google schema")
	}

	roundTrip, err := GoogleSchemaToJSONSchema(googleSchema)
	if err != nil {
		t.Fatalf("failed to convert back: %v", err)
	}
	if roundTrip.Type != original.Type {
		t.Errorf("type mismatch: got %s, want %s", roundTrip.Type, original.Type)
	}
}

func TestInferSchemaFromJSON(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		expectedType string
	}{
		{
			name:         "simple object",
			json:         `{"name": "Alice", "age": 30}`,
			expectedType: "object",
		},
		{
			name:         "array",
			json:         `[1, 2, 3]`,
			expectedType: "array",
		},
		{
			name:         "string",
			json:         `"hello"`,
			expectedType: "string",
		},
		{
			name:         "number",
			json:         `3.14`,
			expectedType: "number",
		},
		{
			name:         "integer",
			json:         `42`,
			expectedType: "integer",
		},
		{
			name:         "boolean",
			json:         `true`,
			expectedType: "boolean",
		},
		{
			name:         "null",
			json:         `null`,
			expectedType: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := InferSchemaFromJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if schema.Type != tt.expectedType {
				t.Errorf("got type %s, want %s", schema.Type, tt.expectedType)
			}
		})
	}
}

func TestInferSchemaFromJSON_NestedObject(t *testing.T) {
	jsonData := `{
		"user": {
			"name": "Alice",
			"scores": [95, 87, 92]
		}
	}`

	schema, err := InferSchemaFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Type != "object" {
		t.Fatalf("expected object, got %s", schema.Type)
	}

	userProp, ok := schema.Properties["user"]
	if !ok {
		t.Fatal("missing 'user' property")
	}
	if userProp.Type != "object" {
		t.Errorf("expected user to be object, got %s", userProp.Type)
	}

	scoresProp, ok := userProp.Properties["scores"]
	if !ok {
		t.Fatal("missing 'scores' property")
	}
	if scoresProp.Type != "array" {
		t.Errorf("expected scores to be array, got %s", scoresProp.Type)
	}
}

func TestSchemaJSONRoundTrip(t *testing.T) {
	// Test that our JSONSchema type serializes/deserializes correctly
	minLen := 5
	maxLen := 100
	min := 1.0
	max := 10.0

	original := &JSONSchema{
		Type: "object",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:      "string",
				MinLength: &minLen,
				MaxLength: &maxLen,
			},
			"score": {
				Type:    "number",
				Minimum: &min,
				Maximum: &max,
			},
			"tags": {
				Type: "array",
				Items: &JSONSchema{
					Type: "string",
					Enum: []string{"a", "b", "c"},
				},
			},
		},
		Required: []string{"name", "score"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed JSONSchema
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Type != "object" {
		t.Errorf("type mismatch: got %s", parsed.Type)
	}
	if len(parsed.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(parsed.Properties))
	}
	if len(parsed.Required) != 2 {
		t.Errorf("expected 2 required, got %d", len(parsed.Required))
	}
}
