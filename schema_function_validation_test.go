package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaCompileFunctionValidation(t *testing.T) {
	rootAssert := assert.New(t)
	schema := functionValidationSchema()
	cs, err := CompileSchema(schema)
	rootAssert.NoError(err)
	rootAssert.NotNil(cs)

	modes := []struct {
		name    string
		compile func(string) (*JMESPath, error)
	}{
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

	tests := []struct {
		expression string
		code       string
		offset     int
	}{
		{expression: "unknown(name)", code: staticErrUnknownFunction, offset: 0},
		{expression: "length(name, name)", code: staticErrInvalidFuncArity, offset: 0},
		{expression: "contains(name, 'x')"},
		{expression: "contains(items, 'x')"},
		{expression: "contains(price, 'x')", code: staticErrInvalidFuncArgType, offset: 9},
		{expression: "abs(name)", code: staticErrInvalidFuncArgType, offset: 4},
		{expression: "sum(values)", code: staticErrUnverifiableType, offset: 4},
		{expression: "max_by(items, &name)"},
		{expression: "min_by(items, &name)"},
		{expression: "sort_by(items, &name)"},
		{expression: "sort_by(items, &active)", code: staticErrInvalidFuncArgType, offset: 15},
		{expression: "sort_by(items, &scores[0])", code: staticErrUnverifiableType, offset: 15},
		{expression: "merge(meta, meta)"},
		{expression: "merge(name)", code: staticErrInvalidFuncArgType, offset: 6},
		{expression: "not_null()", code: staticErrInvalidFuncArity, offset: 0},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			modeAssert := assert.New(t)
			for _, tt := range tests {
				jp, err := mode.compile(tt.expression)
				if tt.code == "" {
					modeAssert.NoError(err, tt.expression)
					modeAssert.NotNil(jp, tt.expression)
					continue
				}
				modeAssert.Error(err, tt.expression)
				var staticErr *StaticError
				modeAssert.ErrorAs(err, &staticErr, tt.expression)
				modeAssert.Equal(tt.code, staticErr.Code, tt.expression)
				modeAssert.Equal(tt.offset, staticErr.Offset, tt.expression)
			}
		})
	}
}

func functionValidationSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"price": map[string]interface{}{
				"type": "number",
			},
			"values": map[string]interface{}{
				"type": "array",
			},
			"meta": map[string]interface{}{
				"type": "object",
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"active": map[string]interface{}{
							"type": "boolean",
						},
						"scores": map[string]interface{}{
							"type": "array",
						},
					},
					"additionalProperties": false,
				},
			},
		},
		"additionalProperties": false,
	}
}
