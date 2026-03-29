package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompileWithSchemaEnumAwareLiteralValidation(t *testing.T) {
	assert := assert.New(t)
	schema := enumAwareTestSchema()
	tests := []struct {
		expression string
		code       string
	}{
		{expression: "status == 'active'"},
		{expression: "status == 'actvie'", code: staticErrInvalidEnumValue},
		{expression: "status != 'actvie'", code: staticErrInvalidEnumValue},
		{expression: "'active' == status"},
		{expression: "'actvie' == status", code: staticErrInvalidEnumValue},
		{expression: "priority == `2`"},
		{expression: "priority == `5`", code: staticErrInvalidEnumValue},
		{expression: "enabled == `true`"},
		{expression: "enabled == `false`", code: staticErrInvalidEnumValue},
		{expression: "contains(tags, 'prod')"},
		{expression: "contains(tags, 'prodution')", code: staticErrInvalidEnumValue},
		{expression: "contains(title, 'pro')"},
		{expression: "name == 'Alice'"},
	}

	for _, tt := range tests {
		_, err := CompileWithSchema(tt.expression, schema)
		if tt.code == "" {
			assert.NoError(err, tt.expression)
			continue
		}
		assert.Error(err, tt.expression)
		var staticErr *StaticError
		assert.ErrorAs(err, &staticErr, tt.expression)
		assert.Equal(tt.code, staticErr.Code, tt.expression)
	}
}

func TestCompileWithCompiledSchemaEnumAwareLiteralValidation(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(enumAwareTestSchema())
	assert.NoError(err)
	assert.NotNil(cs)

	_, err = CompileWithCompiledSchema("status == 'actvie'", cs)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrInvalidEnumValue, staticErr.Code)
	assert.Equal(11, staticErr.Offset)
	assert.Contains(staticErr.Message, "field status")
}

func TestCompileSchemaMixedTypeEnumUnsupported(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"enum": []interface{}{"active", 1},
			},
		},
	}

	_, err := CompileSchema(schema)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsupportedSchema, staticErr.Code)
}

func TestCompileSchemaObjectEnumUnsupported(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"enum": []interface{}{map[string]interface{}{"value": "active"}},
			},
		},
	}

	_, err := CompileSchema(schema)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsupportedSchema, staticErr.Code)
}

func enumAwareTestSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"active", "archived"},
			},
			"priority": map[string]interface{}{
				"type": "number",
				"enum": []interface{}{1, 2, 3},
			},
			"enabled": map[string]interface{}{
				"type":  "boolean",
				"const": true,
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"prod", "staging", "dev"},
				},
			},
			"title": map[string]interface{}{
				"type": "string",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"required":             []interface{}{"status", "priority", "enabled", "tags", "title", "name"},
		"additionalProperties": false,
	}
}
