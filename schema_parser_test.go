package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompileSchemaSupportedSubset(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type": "string",
			},
			"dynamic": map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type": "number",
				},
				"title":       "ignored",
				"description": "ignored",
				"default":     "ignored",
				"examples":    []interface{}{"ignored"},
			},
		},
		"required":             []interface{}{"foo"},
		"additionalProperties": false,
	}

	compiled, err := CompileSchema(schema)
	assert.NoError(err)
	assert.NotNil(compiled)
	assert.NotNil(compiled.root)
	assert.NotNil(compiled.staticRoot)
}

func TestCompileSchemaUnsupportedKeywords(t *testing.T) {
	assert := assert.New(t)
	keywords := []string{
		"$ref",
		"oneOf",
		"anyOf",
		"allOf",
		"if",
		"then",
		"else",
		"prefixItems",
		"patternProperties",
		"unevaluatedProperties",
	}

	for _, keyword := range keywords {
		schema := JSONSchema{
			"type": "object",
			keyword: []interface{}{
				map[string]interface{}{"type": "string"},
			},
		}
		_, err := CompileSchema(schema)
		assert.Error(err, keyword)
		var staticErr *StaticError
		assert.ErrorAs(err, &staticErr, keyword)
		assert.Equal(staticErrUnsupportedSchema, staticErr.Code, keyword)
	}
}

func TestCompileSchemaTypeArrayIsUnsupported(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": []interface{}{"object", "null"},
	}

	_, err := CompileSchema(schema)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsupportedSchema, staticErr.Code)
}
