package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompileSchemaSupportsOneOf(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"title": "root",
		"oneOf": []interface{}{
			map[string]interface{}{"type": "string"},
			map[string]interface{}{"type": "number"},
		},
	}

	compiled, err := CompileSchema(schema)
	assert.NoError(err)
	if !assert.NotNil(compiled) || !assert.NotNil(compiled.root) || !assert.NotNil(compiled.staticRoot) {
		return
	}

	assert.Len(compiled.root.oneOf, 2)
	assert.Equal(staticMaskString|staticMaskNumber, compiled.staticRoot.mask)
}

func TestCompileSchemaRejectsInvalidOneOfShape(t *testing.T) {
	tests := []struct {
		name            string
		schema          JSONSchema
		expectedPath    string
		expectedMessage string
	}{
		{
			name: "not array",
			schema: JSONSchema{
				"oneOf": map[string]interface{}{"type": "string"},
			},
			expectedPath:    "$",
			expectedMessage: "oneOf must be a non-empty array of schema objects",
		},
		{
			name: "empty array",
			schema: JSONSchema{
				"oneOf": []interface{}{},
			},
			expectedPath:    "$",
			expectedMessage: "oneOf must be a non-empty array of schema objects",
		},
		{
			name: "non object branch",
			schema: JSONSchema{
				"oneOf": []interface{}{
					map[string]interface{}{"type": "string"},
					"string",
				},
			},
			expectedPath:    "$",
			expectedMessage: "oneOf[1] must be a schema object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUnsupportedSchemaCompileError(t, tt.schema, tt.expectedPath, tt.expectedMessage)
		})
	}
}

func TestCompileSchemaRejectsOneOfSiblingConstraints(t *testing.T) {
	schema := JSONSchema{
		"oneOf": []interface{}{
			map[string]interface{}{"type": "string"},
			map[string]interface{}{"type": "number"},
		},
		"type": "string",
	}

	assertUnsupportedSchemaCompileError(t, schema, "$", "sibling keyword")
}
