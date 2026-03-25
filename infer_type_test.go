package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferTypeWithCompiledSchemaSuccessCases(t *testing.T) {
	a := assert.New(t)
	cs, err := CompileSchema(inferTypeTestSchema())
	a.NoError(err)
	a.NotNil(cs)

	tests := []struct {
		expression string
		assertType func(*assert.Assertions, *InferredType)
	}{
		{
			expression: "name",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
				a.Nil(typ.Item)
				a.Nil(typ.Properties)
			},
		},
		{
			expression: "items[0].price",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber, typ.Mask)
			},
		},
		{
			expression: "values",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeAny, typ.Item.Mask)
				a.Nil(typ.Item.Item)
				a.NotSame(typ, typ.Item)
			},
		},
		{
			expression: "items[0:2]",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeObject, typ.Item.Mask)
				a.False(typ.Item.OpenObject)
				a.Contains(typ.Item.Properties, "price")
				a.Equal(TypeNumber, typ.Item.Properties["price"].Mask)
			},
		},
		{
			expression: "items[].price",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeNumber, typ.Item.Mask)
			},
		},
		{
			expression: "items[?price > `10`].name",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeString, typ.Item.Mask)
			},
		},
		{
			expression: "[name, items[0].price]",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeString|TypeNumber, typ.Item.Mask)
			},
		},
		{
			expression: "{name: name, price: items[0].price}",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.False(typ.OpenObject)
				a.Len(typ.Properties, 2)
				a.Equal(TypeString, typ.Properties["name"].Mask)
				a.Equal(TypeNumber, typ.Properties["price"].Mask)
			},
		},
		{
			expression: "length(name)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber, typ.Mask)
			},
		},
		{
			expression: "contains(name, 'x')",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeBoolean, typ.Mask)
			},
		},
		{
			expression: "type(name)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
			},
		},
		{
			expression: "max_by(items, &price)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.False(typ.OpenObject)
				a.Contains(typ.Properties, "price")
			},
		},
		{
			expression: "sort_by(items, &price)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeObject, typ.Item.Mask)
			},
		},
		{
			expression: "map(&price, items)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeNumber, typ.Item.Mask)
			},
		},
		{
			expression: "name || items[0].price",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString|TypeNumber, typ.Mask)
			},
		},
		{
			expression: "openObj",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.True(typ.OpenObject)
				a.Contains(typ.Properties, "known")
				a.Equal(TypeNumber, typ.Properties["known"].Mask)
			},
		},
		{
			expression: "closedObj",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.False(typ.OpenObject)
				a.Contains(typ.Properties, "id")
				a.Equal(TypeString, typ.Properties["id"].Mask)
			},
		},
		{
			expression: "status",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
				a.Equal([]interface{}{"active", "archived"}, typ.Enum)
				a.Nil(typ.Const)
			},
		},
		{
			expression: "currency",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
				a.Equal("USD", typ.Const)
				a.Nil(typ.Enum)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			subAssert := assert.New(t)
			typeResult, inferErr := InferTypeWithCompiledSchema(tt.expression, cs)
			subAssert.NoError(inferErr)
			subAssert.NotNil(typeResult)
			tt.assertType(subAssert, typeResult)
		})
	}
}

func TestInferTypeWithSchemaMatchesCompiledSchemaPath(t *testing.T) {
	a := assert.New(t)
	schema := inferTypeTestSchema()
	cs, err := CompileSchema(schema)
	a.NoError(err)

	withSchema, err := InferTypeWithSchema("items[].price", schema)
	a.NoError(err)
	withCompiled, err := InferTypeWithCompiledSchema("items[].price", cs)
	a.NoError(err)
	a.Equal(withSchema, withCompiled)
}

func TestInferTypeErrors(t *testing.T) {
	a := assert.New(t)
	schema := inferTypeTestSchema()
	cs, err := CompileSchema(schema)
	a.NoError(err)

	modes := []struct {
		name  string
		infer func(string) (*InferredType, error)
	}{
		{
			name: "InferTypeWithSchema",
			infer: func(expression string) (*InferredType, error) {
				return InferTypeWithSchema(expression, schema)
			},
		},
		{
			name: "InferTypeWithCompiledSchema",
			infer: func(expression string) (*InferredType, error) {
				return InferTypeWithCompiledSchema(expression, cs)
			},
		},
	}

	tests := []struct {
		expression string
		code       string
		offset     int
	}{
		{expression: "closedObj.unknown", code: staticErrUnknownProperty, offset: 10},
		{expression: "name.foo", code: staticErrInvalidFieldTarget, offset: 5},
		{expression: "name[0]", code: staticErrInvalidIndexTarget, offset: 4},
		{expression: "name[]", code: staticErrInvalidProjection, offset: 4},
		{expression: "abs(name)", code: staticErrInvalidFuncArgType, offset: 4},
		{expression: "sum(values)", code: staticErrUnverifiableType, offset: 4},
		{expression: "openObj.unknown", code: staticErrUnverifiableProperty, offset: 8},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			subAssert := assert.New(t)
			for _, tt := range tests {
				result, inferErr := mode.infer(tt.expression)
				subAssert.Error(inferErr, tt.expression)
				subAssert.Nil(result, tt.expression)

				var staticErr *StaticError
				subAssert.ErrorAs(inferErr, &staticErr, tt.expression)
				subAssert.Equal(tt.code, staticErr.Code, tt.expression)
				subAssert.Equal(tt.offset, staticErr.Offset, tt.expression)
			}
		})
	}
}

func TestInferTypeWithCompiledSchemaNil(t *testing.T) {
	a := assert.New(t)
	result, err := InferTypeWithCompiledSchema("name", nil)
	a.Error(err)
	a.Nil(result)
	var staticErr *StaticError
	a.ErrorAs(err, &staticErr)
	a.Equal(staticErrUnsupportedSchema, staticErr.Code)
}

func inferTypeTestSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"name":   map[string]interface{}{"type": "string"},
			"values": map[string]interface{}{"type": "array"},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price":  map[string]interface{}{"type": "number"},
						"name":   map[string]interface{}{"type": "string"},
						"active": map[string]interface{}{"type": "boolean"},
						"tags": map[string]interface{}{
							"type":  "array",
							"items": map[string]interface{}{"type": "string"},
						},
					},
					"additionalProperties": false,
				},
			},
			"openObj": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"known": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": true,
			},
			"closedObj": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
			},
			"status": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"active", "archived"},
			},
			"currency": map[string]interface{}{
				"type":  "string",
				"const": "USD",
			},
		},
		"additionalProperties": false,
	}
}
