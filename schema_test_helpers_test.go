package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type schemaCompileMode struct {
	name    string
	compile func(string) (*JMESPath, error)
}

func compileSchemaForTest(t *testing.T, schema JSONSchema) *CompiledSchema {
	t.Helper()

	cs, err := CompileSchema(schema)
	if err != nil {
		t.Fatalf("CompileSchema() unexpected error: %v", err)
	}
	if cs == nil {
		t.Fatal("CompileSchema() returned nil compiled schema")
	}
	return cs
}

func schemaCompileModes(t *testing.T, schema JSONSchema) []schemaCompileMode {
	t.Helper()

	cs := compileSchemaForTest(t, schema)
	return []schemaCompileMode{
		{
			name: "CompileWithSchema",
			compile: func(expression string) (*JMESPath, error) {
				return CompileWithSchema(expression, schema)
			},
		},
		{
			name: "CompileWithCompiledSchema",
			compile: func(expression string) (*JMESPath, error) {
				return CompileWithCompiledSchema(expression, cs)
			},
		},
	}
}

func assertStaticErrorCode(t *testing.T, err error, expectedCode, expression string) {
	t.Helper()

	assert.Error(t, err, expression)
	var staticErr *StaticError
	if !assert.ErrorAs(t, err, &staticErr, expression) {
		return
	}
	assert.Equal(t, expectedCode, staticErr.Code, expression)
}

func compileTestSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bar": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"baz": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "number",
								},
							},
						},
						"additionalProperties": false,
					},
				},
				"additionalProperties": false,
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price": map[string]interface{}{
							"type": "number",
						},
						"name": map[string]interface{}{
							"type": "string",
						},
					},
					"additionalProperties": false,
				},
			},
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"additionalProperties": false,
	}
}

func compileTestSchemaWithRequired(required ...string) JSONSchema {
	schema := compileTestSchema()
	if len(required) == 0 {
		return schema
	}
	items := make([]interface{}, len(required))
	for i, name := range required {
		items[i] = name
	}
	schema["required"] = items
	return schema
}

func functionNullableSafetySchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sections": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"summary": map[string]interface{}{
								"type": "string",
							},
						},
						"additionalProperties": false,
					},
				},
				"required":             []interface{}{"sections"},
				"additionalProperties": false,
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"records": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
						"label": map[string]interface{}{
							"type": "string",
						},
						"rank": map[string]interface{}{
							"type": "number",
						},
					},
					"required":             []interface{}{"id"},
					"additionalProperties": false,
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type": "string",
					},
					"status": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []interface{}{"status"},
				"additionalProperties": false,
			},
			"numbers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "number",
				},
			},
			"optional_numbers": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "number",
				},
			},
			"mixed_array": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"optional_mixed_array": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"optional_path": map[string]interface{}{
				"type": "string",
			},
			"optional_number": map[string]interface{}{
				"type": "number",
			},
			"null_field": map[string]interface{}{
				"type": "string",
			},
		},
		"required":             []interface{}{"items", "records", "meta", "numbers", "mixed_array"},
		"additionalProperties": false,
	}
}
