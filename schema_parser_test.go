package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertUnsupportedSchemaCompileError(t *testing.T, schema JSONSchema, expectedPath, expectedKeyword string) {
	t.Helper()
	_, err := CompileSchema(schema)
	assert.Error(t, err)
	var staticErr *StaticError
	assert.ErrorAs(t, err, &staticErr)
	assert.Equal(t, staticErrUnsupportedSchema, staticErr.Code)
	assert.Contains(t, staticErr.Message, expectedPath)
	assert.Contains(t, staticErr.Message, expectedKeyword)
}

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

func TestCompileSchemaUnknownKeywordAtRoot(t *testing.T) {
	schema := JSONSchema{
		"type":           "object",
		"unknownKeyword": true,
	}
	assertUnsupportedSchemaCompileError(t, schema, "$", "unknownKeyword")
}

func TestCompileSchemaMisspelledAdditionalPropertiesKeywordAtRoot(t *testing.T) {
	schema := JSONSchema{
		"type":                 "object",
		"additionalProperites": false,
	}
	assertUnsupportedSchemaCompileError(t, schema, "$", "additionalProperites")
}

func TestCompileSchemaUnknownKeywordInPropertySchema(t *testing.T) {
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type":  "string",
				"typoe": "string",
			},
		},
	}
	assertUnsupportedSchemaCompileError(t, schema, "$.properties.foo", "typoe")
}

func TestCompileSchemaUnknownKeywordInItemsSchema(t *testing.T) {
	schema := JSONSchema{
		"type": "array",
		"items": map[string]interface{}{
			"type":  "string",
			"typoe": "string",
		},
	}
	assertUnsupportedSchemaCompileError(t, schema, "$.items", "typoe")
}

func TestCompileSchemaUnknownKeywordInAdditionalPropertiesSchema(t *testing.T) {
	schema := JSONSchema{
		"type": "object",
		"additionalProperties": map[string]interface{}{
			"type":  "string",
			"typoe": "string",
		},
	}
	assertUnsupportedSchemaCompileError(t, schema, "$.additionalProperties", "typoe")
}

func TestCompileSchemaMetadataKeywordsAreIgnored(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type":        "object",
		"title":       "root-title",
		"description": "root-description",
		"default":     "root-default",
		"examples":    []interface{}{"root-example"},
		"properties": map[string]interface{}{
			"foo": map[string]interface{}{
				"type":        "array",
				"title":       "foo-title",
				"description": "foo-description",
				"default":     []interface{}{},
				"examples":    []interface{}{[]interface{}{}},
				"items": map[string]interface{}{
					"type":        "string",
					"title":       "item-title",
					"description": "item-description",
					"default":     "item-default",
					"examples":    []interface{}{"item-example"},
				},
			},
		},
		"additionalProperties": map[string]interface{}{
			"type":        "string",
			"title":       "additional-title",
			"description": "additional-description",
			"default":     "additional-default",
			"examples":    []interface{}{"additional-example"},
		},
	}

	compiled, err := CompileSchema(schema)
	assert.NoError(err)
	assert.NotNil(compiled)
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

func TestCompileSchemaDefaultsAdditionalPropertiesToOpen(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"known": map[string]interface{}{"type": "string"},
		},
	}

	compiled, err := CompileSchema(schema)
	assert.NoError(err)
	assert.NotNil(compiled)
	assert.NotNil(compiled.root)
	assert.Equal(additionalPropertiesAllowOpen, compiled.root.additionalPropertiesMode)
	assert.Nil(compiled.root.additionalPropertiesSchema)
	assert.NotNil(compiled.staticRoot)
	assert.NotNil(compiled.staticRoot.object)
	assert.Equal(additionalPropertiesAllowOpen, compiled.staticRoot.object.additionalMode)
	assert.Nil(compiled.staticRoot.object.additionalSchema)
}
