package rlm

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// SchemaValidator provides JSON Schema validation using Google's jsonschema-go package.
// It wraps Google's library to provide proper Draft 2020-12 compliant validation
// while maintaining backward compatibility with our existing JSONSchema type.
type SchemaValidator struct {
	resolved *jsonschema.Resolved
	raw      *JSONSchema
}

// NewSchemaValidator creates a SchemaValidator from our internal JSONSchema type.
// It converts the internal representation to a Google jsonschema.Schema,
// resolves it, and prepares it for validation.
func NewSchemaValidator(schema *JSONSchema) (*SchemaValidator, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	// Convert our JSONSchema to a standard JSON representation
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Parse into Google's Schema type
	var googleSchema jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &googleSchema); err != nil {
		return nil, fmt.Errorf("failed to parse schema into Google format: %w", err)
	}

	// Resolve the schema for validation
	resolved, err := googleSchema.Resolve(nil)
	if err != nil {
		// Fall back to basic validation if resolution fails
		// This can happen with simple schemas that don't need full resolution
		return &SchemaValidator{
			resolved: nil,
			raw:      schema,
		}, nil
	}

	return &SchemaValidator{
		resolved: resolved,
		raw:      schema,
	}, nil
}

// Validate checks data against the JSON schema using Google's validator.
// Falls back to our internal validation if the Google validator isn't available.
func (sv *SchemaValidator) Validate(data interface{}) error {
	if sv.resolved != nil {
		if err := sv.resolved.Validate(data); err != nil {
			return fmt.Errorf("schema validation failed: %w", err)
		}
		return nil
	}

	// Fallback to internal validation for simple schemas
	if dataMap, ok := data.(map[string]interface{}); ok {
		return validateAgainstSchema(dataMap, sv.raw)
	}
	return validateValue(data, sv.raw)
}

// ValidateJSON validates a JSON byte slice against the schema.
func (sv *SchemaValidator) ValidateJSON(jsonData []byte) error {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return sv.Validate(data)
}

// JSONSchemaToGoogleSchema converts our internal JSONSchema to Google's Schema type.
// This is useful for operations that need the Google type directly.
func JSONSchemaToGoogleSchema(schema *JSONSchema) (*jsonschema.Schema, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var googleSchema jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &googleSchema); err != nil {
		return nil, fmt.Errorf("failed to parse into Google schema: %w", err)
	}

	return &googleSchema, nil
}

// GoogleSchemaToJSONSchema converts a Google Schema back to our internal type.
func GoogleSchemaToJSONSchema(schema *jsonschema.Schema) (*JSONSchema, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Google schema: %w", err)
	}

	var internalSchema JSONSchema
	if err := json.Unmarshal(schemaBytes, &internalSchema); err != nil {
		return nil, fmt.Errorf("failed to parse into internal schema: %w", err)
	}

	return &internalSchema, nil
}

// InferSchemaFromJSON infers a JSON Schema from a JSON example.
// This is useful for the meta-agent to generate schemas from example data.
func InferSchemaFromJSON(jsonData []byte) (*JSONSchema, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return inferSchemaFromValue(data), nil
}

// inferSchemaFromValue recursively builds a JSONSchema from a Go value.
func inferSchemaFromValue(value interface{}) *JSONSchema {
	if value == nil {
		return &JSONSchema{Type: "null"}
	}

	switch v := value.(type) {
	case bool:
		return &JSONSchema{Type: "boolean"}
	case float64:
		// JSON numbers are float64 by default
		if v == float64(int64(v)) {
			return &JSONSchema{Type: "integer"}
		}
		return &JSONSchema{Type: "number"}
	case string:
		return &JSONSchema{Type: "string"}
	case []interface{}:
		schema := &JSONSchema{Type: "array"}
		if len(v) > 0 {
			schema.Items = inferSchemaFromValue(v[0])
		}
		return schema
	case map[string]interface{}:
		schema := &JSONSchema{
			Type:       "object",
			Properties: make(map[string]*JSONSchema),
			Required:   make([]string, 0),
		}
		for key, val := range v {
			schema.Properties[key] = inferSchemaFromValue(val)
			schema.Required = append(schema.Required, key)
		}
		return schema
	default:
		return &JSONSchema{Type: "string"}
	}
}
