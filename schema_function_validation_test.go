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

func TestSchemaCompileFunctionValidationNullableSafety(t *testing.T) {
	rootAssert := assert.New(t)
	schema := functionNullableSafetySchema()
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

	errorCases := []struct {
		expression       string
		code             string
		messageContains  string
		messageContains2 string
	}{
		{
			expression:       "contains(content.sections.summary, 'retry')",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "contains"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "contains(items[10], 'retry')",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "contains"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "length(meta.title)",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "length"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "starts_with(meta.title, 'x')",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "starts_with"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "ends_with(meta.title, 'x')",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "ends_with"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "contains(null_field, 'x') || 'fallback'",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "contains"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "sum(optional_numbers)",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "sum"`,
			messageContains2: "argument 1",
		},
		{
			expression:       "max_by(records, &rank)",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "max_by"`,
			messageContains2: "argument 2",
		},
		{
			expression:       "min_by(records, &label)",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "min_by"`,
			messageContains2: "argument 2",
		},
		{
			expression:       "sort_by(records, &label)",
			code:             staticErrUnsafeOptionalArg,
			messageContains:  `function "sort_by"`,
			messageContains2: "argument 2",
		},
		{
			expression:      "sum(mixed_array)",
			code:            staticErrInvalidFuncArgType,
			messageContains: `function "sum"`,
		},
		{
			expression:      "sum(optional_mixed_array)",
			code:            staticErrInvalidFuncArgType,
			messageContains: `function "sum"`,
		},
	}

	successCases := []string{
		"content.sections.summary && contains(content.sections.summary, 'retry')",
		"contains(not_null(content.sections.summary, ''), 'retry')",
		"null_field && contains(null_field, 'x')",
		"not_null('fallback', contains(meta.title, 'x'))",
		"meta.status == 'active'",
		"optional_path || 'fallback'",
		"items[10] != `null` && contains(items[10], 'retry')",
		"optional_number > `4`",
		"content.sections.summary",
		"items[10]",
		"sum(numbers)",
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			modeAssert := assert.New(t)
			for _, tt := range errorCases {
				jp, compileErr := mode.compile(tt.expression)
				modeAssert.Error(compileErr, tt.expression)
				modeAssert.Nil(jp, tt.expression)
				var staticErr *StaticError
				modeAssert.ErrorAs(compileErr, &staticErr, tt.expression)
				modeAssert.Equal(tt.code, staticErr.Code, tt.expression)
				if tt.messageContains != "" {
					modeAssert.Contains(staticErr.Message, tt.messageContains, tt.expression)
				}
				if tt.messageContains2 != "" {
					modeAssert.Contains(staticErr.Message, tt.messageContains2, tt.expression)
				}
			}
			for _, expression := range successCases {
				jp, compileErr := mode.compile(expression)
				modeAssert.NoError(compileErr, expression)
				modeAssert.NotNil(jp, expression)
			}
		})
	}
}

func TestSchemaCompileFunctionValidationNullableFunctionReturns(t *testing.T) {
	rootAssert := assert.New(t)
	schema := functionNullableReturnSchema()
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
		{expression: "abs(to_number(name))", code: staticErrUnsafeOptionalArg, offset: 4},
		{expression: "sum(map(&to_number(name), items))", code: staticErrUnsafeOptionalArg, offset: 4},
		{expression: "sort_by(items, &to_number(name))", code: staticErrUnsafeOptionalArg, offset: 15},
		{expression: "max_by(items, &max(scores))", code: staticErrUnsafeOptionalArg, offset: 14},
		{expression: "min_by(items, &min(scores))", code: staticErrUnsafeOptionalArg, offset: 14},
		{expression: "abs(to_number(count))"},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			modeAssert := assert.New(t)
			for _, tt := range tests {
				jp, compileErr := mode.compile(tt.expression)
				if tt.code == "" {
					modeAssert.NoError(compileErr, tt.expression)
					modeAssert.NotNil(jp, tt.expression)
					continue
				}
				modeAssert.Error(compileErr, tt.expression)
				modeAssert.Nil(jp, tt.expression)
				var staticErr *StaticError
				modeAssert.ErrorAs(compileErr, &staticErr, tt.expression)
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
					"required":             []interface{}{"name", "active", "scores"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []interface{}{"name", "price", "values", "meta", "items"},
		"additionalProperties": false,
	}
}

func functionNullableReturnSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"count": map[string]interface{}{
				"type": "number",
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"scores": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "number",
							},
						},
					},
					"required":             []interface{}{"name", "scores"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []interface{}{"name", "count", "items"},
		"additionalProperties": false,
	}
}
