package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOneOfObjectUnionBehavior(t *testing.T) {
	schema := oneOfEntitySchema()

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			assert := assert.New(t)

			jp, err := mode.compile("kind")
			assert.NoError(err)
			assert.NotNil(jp)

			jp, err = mode.compile("name")
			assert.NoError(err)
			assert.NotNil(jp)

			jp, err = mode.compile("kind == 'user'")
			assert.NoError(err)
			assert.NotNil(jp)

			_, err = mode.compile("kind == 'test'")
			assertStaticErrorCode(t, err, staticErrInvalidEnumValue, "kind == 'test'")

			_, err = mode.compile("unknown")
			assertStaticErrorCode(t, err, staticErrUnknownProperty, "unknown")

			_, err = mode.compile("starts_with(name, 'A')")
			assertStaticErrorCode(t, err, staticErrUnsafeOptionalArg, "starts_with(name, 'A')")
		})
	}

	kind, err := InferTypeWithSchema("kind", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, kind) {
		return
	}
	assert.Equal(t, TypeString, kind.Mask)
	assert.Equal(t, []interface{}{"user", "org"}, kind.Enum)
	assert.Nil(t, kind.Const)

	name, err := InferTypeWithSchema("name", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, name) {
		return
	}
	assert.Equal(t, TypeString|TypeNull, name.Mask)
}

func TestOneOfOpenObjectPropertyIsUnverifiable(t *testing.T) {
	schema := JSONSchema{
		"oneOf": []interface{}{
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []interface{}{"name"},
				"additionalProperties": false,
			},
			map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			_, err := mode.compile("name")
			assertStaticErrorCode(t, err, staticErrUnverifiableProperty, "name")
		})
	}
}

func TestOneOfTypedAdditionalPropertiesContributeToFieldType(t *testing.T) {
	schema := JSONSchema{
		"oneOf": []interface{}{
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []interface{}{"name"},
				"additionalProperties": false,
			},
			map[string]interface{}{
				"type":                 "object",
				"additionalProperties": map[string]interface{}{"type": "number"},
			},
		},
	}

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			jp, err := mode.compile("name")
			assert.NoError(t, err)
			assert.NotNil(t, jp)

			jp, err = mode.compile("other")
			assert.NoError(t, err)
			assert.NotNil(t, jp)
		})
	}

	nameType, err := InferTypeWithSchema("name", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, nameType) {
		return
	}
	assert.Equal(t, TypeString|TypeNumber|TypeNull, nameType.Mask)

	otherType, err := InferTypeWithSchema("other", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, otherType) {
		return
	}
	assert.Equal(t, TypeNumber|TypeNull, otherType.Mask)
}

func TestOneOfArrayBehavior(t *testing.T) {
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "number"},
					},
				},
			},
		},
		"required":             []interface{}{"items"},
		"additionalProperties": false,
	}

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			assert := assert.New(t)

			jp, err := mode.compile("items[0]")
			assert.NoError(err)
			assert.NotNil(jp)

			jp, err = mode.compile("items[]")
			assert.NoError(err)
			assert.NotNil(jp)
		})
	}

	indexType, err := InferTypeWithSchema("items[0]", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, indexType) {
		return
	}
	assert.Equal(t, TypeString|TypeNumber|TypeNull, indexType.Mask)

	projectionType, err := InferTypeWithSchema("items[]", schema)
	if !assert.NoError(t, err) || !assert.NotNil(t, projectionType) {
		return
	}
	assert.Equal(t, TypeArray, projectionType.Mask)
	if assert.NotNil(t, projectionType.Item) {
		assert.Equal(t, TypeString|TypeNumber, projectionType.Item.Mask)
	}
}

func TestOneOfConstWithNullAllowsNullLiteralComparator(t *testing.T) {
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{"const": "x"},
					map[string]interface{}{"type": "null"},
				},
			},
		},
		"required":             []interface{}{"value"},
		"additionalProperties": false,
	}

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			jp, err := mode.compile("value == `null`")
			assert.NoError(t, err)
			assert.NotNil(t, jp)

			jp, err = mode.compile("value == 'x'")
			assert.NoError(t, err)
			assert.NotNil(t, jp)

			_, err = mode.compile("value == 'y'")
			assertStaticErrorCode(t, err, staticErrInvalidEnumValue, "value == 'y'")
		})
	}
}

func TestOneOfTypedAdditionalPropertiesRemainUnverifiableForUnknownField(t *testing.T) {
	schema := JSONSchema{
		"oneOf": []interface{}{
			map[string]interface{}{
				"type":                 "object",
				"additionalProperties": map[string]interface{}{"type": "number"},
			},
			map[string]interface{}{
				"type":                 "object",
				"additionalProperties": map[string]interface{}{"type": "number"},
			},
		},
	}

	for _, mode := range schemaCompileModes(t, schema) {
		t.Run(mode.name, func(t *testing.T) {
			_, err := mode.compile("foo")
			assertStaticErrorCode(t, err, staticErrUnverifiableProperty, "foo")
		})
	}
}

func TestOneOfNestedBehavior(t *testing.T) {
	propertiesSchema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{
				"oneOf": []interface{}{
					map[string]interface{}{"type": "string"},
					map[string]interface{}{"type": "number"},
				},
			},
		},
		"required":             []interface{}{"value"},
		"additionalProperties": false,
	}
	valueType, err := InferTypeWithSchema("value", propertiesSchema)
	if !assert.NoError(t, err) || !assert.NotNil(t, valueType) {
		return
	}
	assert.Equal(t, TypeString|TypeNumber, valueType.Mask)

	itemsSchema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"oneOf": []interface{}{
						map[string]interface{}{"type": "string"},
						map[string]interface{}{"type": "number"},
					},
				},
			},
		},
		"required":             []interface{}{"items"},
		"additionalProperties": false,
	}
	itemType, err := InferTypeWithSchema("items[0]", itemsSchema)
	if !assert.NoError(t, err) || !assert.NotNil(t, itemType) {
		return
	}
	assert.Equal(t, TypeString|TypeNumber|TypeNull, itemType.Mask)

	additionalSchema := JSONSchema{
		"type": "object",
		"additionalProperties": map[string]interface{}{
			"oneOf": []interface{}{
				map[string]interface{}{"type": "string"},
				map[string]interface{}{"type": "number"},
			},
		},
	}
	compiled := compileSchemaForTest(t, additionalSchema)
	if assert.NotNil(t, compiled.staticRoot) && assert.NotNil(t, compiled.staticRoot.object) {
		if assert.NotNil(t, compiled.staticRoot.object.additionalSchema) {
			assert.Equal(t, staticMaskString|staticMaskNumber, compiled.staticRoot.object.additionalSchema.mask)
		}
	}
}

func oneOfEntitySchema() JSONSchema {
	return JSONSchema{
		"oneOf": []interface{}{
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"kind": map[string]interface{}{"const": "user"},
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []interface{}{"kind", "name"},
				"additionalProperties": false,
			},
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"kind":  map[string]interface{}{"const": "org"},
					"title": map[string]interface{}{"type": "string"},
				},
				"required":             []interface{}{"kind", "title"},
				"additionalProperties": false,
			},
		},
	}
}
