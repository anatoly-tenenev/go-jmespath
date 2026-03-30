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
				a.Equal(TypeNumber|TypeNull, typ.Mask)
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
				a.Equal(TypeNumber|TypeNull, typ.Item.Mask)
			},
		},
		{
			expression: "items[?price > `10`].name",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeString|TypeNull, typ.Item.Mask)
			},
		},
		{
			expression: "[name, items[0].price]",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeString|TypeNumber|TypeNull, typ.Item.Mask)
			},
		},
		{
			expression: "{name: name, price: items[0].price}",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.False(typ.OpenObject)
				a.Len(typ.Properties, 2)
				a.Equal(TypeString, typ.Properties["name"].Mask)
				a.Equal(TypeNumber|TypeNull, typ.Properties["price"].Mask)
			},
		},
		{
			expression: "length(name)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber, typ.Mask)
			},
		},
		{
			expression: "to_number(name)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber|TypeNull, typ.Mask)
			},
		},
		{
			expression: "to_number(count)",
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
			expression: "count > `0`",
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
			expression: "max(numbers)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber|TypeNull, typ.Mask)
			},
		},
		{
			expression: "min(labels)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString|TypeNull, typ.Mask)
			},
		},
		{
			expression: "max_by(items, &not_null(price, `0`))",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject, typ.Mask)
				a.False(typ.OpenObject)
				a.Contains(typ.Properties, "price")
			},
		},
		{
			expression: "sort_by(items, &not_null(price, `0`))",
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
				a.Equal(TypeNumber|TypeNull, typ.Item.Mask)
			},
		},
		{
			expression: "name || items[0].price",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString|TypeNumber|TypeNull, typ.Mask)
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

func TestInferTypeNullableOptionalAndNotNull(t *testing.T) {
	a := assert.New(t)
	cs, err := CompileSchema(inferTypeNullableSchema())
	a.NoError(err)
	a.NotNil(cs)

	tests := []struct {
		expression string
		assertType func(*assert.Assertions, *InferredType)
	}{
		{
			expression: "required_name",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
			},
		},
		{
			expression: "optional_name",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString|TypeNull, typ.Mask)
			},
		},
		{
			expression: "optional_obj",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeObject|TypeNull, typ.Mask)
				a.NotNil(typ.Properties)
				a.Equal(TypeString, typ.Properties["id"].Mask)
			},
		},
		{
			expression: "optional_number > `4`",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeBoolean|TypeNull, typ.Mask)
			},
		},
		{
			expression: "optional_numbers",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray|TypeNull, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeNumber, typ.Item.Mask)
			},
		},
		{
			expression: "numbers[10]",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeNumber|TypeNull, typ.Mask)
			},
		},
		{
			expression: "not_null(optional_name, '')",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
			},
		},
		{
			expression: "not_null(optional_numbers, numbers)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeArray, typ.Mask)
				a.NotNil(typ.Item)
				a.Equal(TypeNumber, typ.Item.Mask)
			},
		},
		{
			expression: "not_null(required_name, optional_numbers)",
			assertType: func(a *assert.Assertions, typ *InferredType) {
				a.Equal(TypeString, typ.Mask)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			subAssert := assert.New(t)
			typ, inferErr := InferTypeWithCompiledSchema(tt.expression, cs)
			subAssert.NoError(inferErr)
			if !subAssert.NotNil(typ) {
				return
			}
			tt.assertType(subAssert, typ)
		})
	}
}

func TestInferredTypeIsXStrictChecks(t *testing.T) {
	tests := []struct {
		name      string
		typ       *InferredType
		isBoolean bool
		isNumber  bool
		isString  bool
		isNull    bool
		isArray   bool
		isObject  bool
	}{
		{
			name:      "boolean",
			typ:       &InferredType{Mask: TypeBoolean},
			isBoolean: true,
		},
		{
			name:     "number",
			typ:      &InferredType{Mask: TypeNumber},
			isNumber: true,
		},
		{
			name:     "string",
			typ:      &InferredType{Mask: TypeString},
			isString: true,
		},
		{
			name:   "null",
			typ:    &InferredType{Mask: TypeNull},
			isNull: true,
		},
		{
			name:    "array",
			typ:     &InferredType{Mask: TypeArray},
			isArray: true,
		},
		{
			name:     "object",
			typ:      &InferredType{Mask: TypeObject},
			isObject: true,
		},
		{
			name: "union_string_number",
			typ:  &InferredType{Mask: TypeString | TypeNumber},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.isBoolean, tt.typ.IsBoolean())
			a.Equal(tt.isNumber, tt.typ.IsNumber())
			a.Equal(tt.isString, tt.typ.IsString())
			a.Equal(tt.isNull, tt.typ.IsNull())
			a.Equal(tt.isArray, tt.typ.IsArray())
			a.Equal(tt.isObject, tt.typ.IsObject())
		})
	}
}

func TestInferredTypeMayBeXChecks(t *testing.T) {
	tests := []struct {
		name         string
		typ          *InferredType
		mayBeBoolean bool
		mayBeNumber  bool
		mayBeString  bool
		mayBeNull    bool
		mayBeArray   bool
		mayBeObject  bool
	}{
		{
			name:         "single_type_boolean",
			typ:          &InferredType{Mask: TypeBoolean},
			mayBeBoolean: true,
		},
		{
			name:         "union_boolean_null",
			typ:          &InferredType{Mask: TypeBoolean | TypeNull},
			mayBeBoolean: true,
			mayBeNull:    true,
		},
		{
			name:        "negative_case_for_boolean",
			typ:         &InferredType{Mask: TypeString | TypeNumber},
			mayBeNumber: true,
			mayBeString: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.mayBeBoolean, tt.typ.MayBeBoolean())
			a.Equal(tt.mayBeNumber, tt.typ.MayBeNumber())
			a.Equal(tt.mayBeString, tt.typ.MayBeString())
			a.Equal(tt.mayBeNull, tt.typ.MayBeNull())
			a.Equal(tt.mayBeArray, tt.typ.MayBeArray())
			a.Equal(tt.mayBeObject, tt.typ.MayBeObject())
		})
	}
}

func TestInferredTypeIsUnion(t *testing.T) {
	tests := []struct {
		name  string
		typ   *InferredType
		union bool
	}{
		{
			name:  "single_type",
			typ:   &InferredType{Mask: TypeString},
			union: false,
		},
		{
			name:  "two_types",
			typ:   &InferredType{Mask: TypeString | TypeNull},
			union: true,
		},
		{
			name:  "several_types",
			typ:   &InferredType{Mask: TypeObject | TypeArray | TypeNumber},
			union: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.union, tt.typ.IsUnion())
		})
	}
}

func TestInferredTypeNilReceiverMethods(t *testing.T) {
	var typ *InferredType
	a := assert.New(t)
	a.False(typ.IsBoolean())
	a.False(typ.IsNumber())
	a.False(typ.IsString())
	a.False(typ.IsNull())
	a.False(typ.IsArray())
	a.False(typ.IsObject())
	a.False(typ.MayBeBoolean())
	a.False(typ.MayBeNumber())
	a.False(typ.MayBeString())
	a.False(typ.MayBeNull())
	a.False(typ.MayBeArray())
	a.False(typ.MayBeObject())
	a.False(typ.IsUnion())
}

func inferTypeTestSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"name":   map[string]interface{}{"type": "string"},
			"count":  map[string]interface{}{"type": "number"},
			"values": map[string]interface{}{"type": "array"},
			"numbers": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "number"},
			},
			"labels": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
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
		"required":             []interface{}{"name", "count", "values", "numbers", "labels", "items", "openObj", "closedObj", "status", "currency"},
		"additionalProperties": false,
	}
}

func inferTypeNullableSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"required_name": map[string]interface{}{
				"type": "string",
			},
			"optional_name": map[string]interface{}{
				"type": "string",
			},
			"optional_number": map[string]interface{}{
				"type": "number",
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
			"optional_obj": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []interface{}{"id"},
				"additionalProperties": false,
			},
		},
		"required":             []interface{}{"required_name", "numbers"},
		"additionalProperties": false,
	}
}
