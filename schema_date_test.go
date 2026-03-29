package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func schemaWithDateFields() JSONSchema {
	return dateFieldSchemaWithRequired("createdDate", "otherDate")
}

func optionalSchemaWithDateFields() JSONSchema {
	return dateFieldSchemaWithRequired()
}

func dateFieldSchemaWithRequired(requiredFields ...string) JSONSchema {
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"createdDate": map[string]interface{}{
				"type":   "string",
				"format": "date",
			},
			"otherDate": map[string]interface{}{
				"type":   "string",
				"format": "date",
			},
			"status": map[string]interface{}{
				"type": "string",
			},
			"count": map[string]interface{}{
				"type": "number",
			},
		},
		"additionalProperties": false,
	}
	if len(requiredFields) != 0 {
		required := make([]interface{}, 0, len(requiredFields))
		for _, field := range requiredFields {
			required = append(required, field)
		}
		schema["required"] = required
	}
	return schema
}

func TestCompileSchemaSupportsStringDateFormat(t *testing.T) {
	assert := assert.New(t)

	compiled, err := CompileSchema(schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(compiled) || !assert.NotNil(compiled.root) || !assert.NotNil(compiled.staticRoot) {
		return
	}

	createdDate := compiled.root.properties["createdDate"]
	assert.NotNil(createdDate)
	assert.Equal(stringFormatDate, createdDate.stringFormat)

	staticCreatedDate := compiled.staticRoot.object.properties["createdDate"]
	assert.NotNil(staticCreatedDate)
	assert.Equal(stringFormatDate, staticCreatedDate.stringFormat)
}

func TestCompileSchemaInfersStringTypeForDateFormatFromConstraints(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		name   string
		schema JSONSchema
	}{
		{
			name: "const",
			schema: JSONSchema{
				"format": "date",
				"const":  "2026-03-01",
			},
		},
		{
			name: "enum",
			schema: JSONSchema{
				"format": "date",
				"enum":   []interface{}{"2026-03-01", "2026-03-02"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileSchema(tt.schema)
			assert.NoError(err)
			if !assert.NotNil(compiled) || !assert.NotNil(compiled.root) || !assert.NotNil(compiled.staticRoot) {
				return
			}

			assert.Equal(schemaKindString, compiled.root.kind)
			assert.Equal(stringFormatDate, compiled.root.stringFormat)
			assert.Equal(stringFormatDate, compiled.staticRoot.stringFormat)
		})
	}
}

func TestCompileSchemaRejectsUnsupportedFormats(t *testing.T) {
	tests := []struct {
		name   string
		schema JSONSchema
	}{
		{
			name: "date format on number",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "number",
						"format": "date",
					},
				},
			},
		},
		{
			name: "date format on boolean",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "boolean",
						"format": "date",
					},
				},
			},
		},
		{
			name: "date format on array",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "array",
						"format": "date",
					},
				},
			},
		},
		{
			name: "date format on object",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "object",
						"format": "date",
					},
				},
			},
		},
		{
			name: "date format on null",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "null",
						"format": "date",
					},
				},
			},
		},
		{
			name: "unknown string format",
			schema: JSONSchema{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type":   "string",
						"format": "date-time",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUnsupportedSchemaCompileError(t, tt.schema, "$.properties.value", "format")
		})
	}
}

func TestCompileSchemaRejectsInvalidDateConstAndEnum(t *testing.T) {
	tests := []struct {
		name   string
		schema JSONSchema
	}{
		{
			name: "invalid date const",
			schema: JSONSchema{
				"type":   "string",
				"format": "date",
				"const":  "draft",
			},
		},
		{
			name: "invalid date enum",
			schema: JSONSchema{
				"type":   "string",
				"format": "date",
				"enum":   []interface{}{"2026-03-01", "draft"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUnsupportedSchemaCompileError(t, tt.schema, "$", "valid date")
		})
	}
}

func TestCompileWithSchemaDateComparators(t *testing.T) {
	assert := assert.New(t)
	schema := schemaWithDateFields()

	successCases := []string{
		"createdDate >= '2026-03-01'",
		"createdDate < otherDate",
	}
	for _, expression := range successCases {
		jp, err := CompileWithSchema(expression, schema)
		assert.NoError(err, expression)
		assert.NotNil(jp, expression)
	}

	errorCases := []struct {
		expression string
	}{
		{expression: "createdDate >= 'draft'"},
		{expression: "createdDate >= '2026-02-30'"},
		{expression: "'2026-03-01' < '2026-03-02'"},
		{expression: "createdDate >= status"},
		{expression: "createdDate >= count"},
		{expression: "createdDate >= `10`"},
	}
	for _, tt := range errorCases {
		_, err := CompileWithSchema(tt.expression, schema)
		assert.Error(err, tt.expression)
		var staticErr *StaticError
		assert.ErrorAs(err, &staticErr, tt.expression)
		assert.Equal(staticErrInvalidComparator, staticErr.Code, tt.expression)
	}
}

func TestCompileWithSchemaOptionalDateComparatorsRequireGuard(t *testing.T) {
	assert := assert.New(t)
	schema := optionalSchemaWithDateFields()

	errorCases := []string{
		"createdDate >= '2026-03-01'",
		"createdDate < otherDate",
	}
	for _, expression := range errorCases {
		_, err := CompileWithSchema(expression, schema)
		assert.Error(err, expression)
		var staticErr *StaticError
		assert.ErrorAs(err, &staticErr, expression)
		assert.Equal(staticErrUnsafeOptionalArg, staticErr.Code, expression)
	}

	successCases := []string{
		"createdDate && createdDate >= '2026-03-01'",
		"createdDate != `null` && createdDate >= '2026-03-01'",
		"createdDate && otherDate && createdDate < otherDate",
		"not_null(createdDate, '2026-03-01') < not_null(otherDate, '2026-03-02')",
	}
	for _, expression := range successCases {
		jp, err := CompileWithSchema(expression, schema)
		assert.NoError(err, expression)
		assert.NotNil(jp, expression)
	}
}

func TestCompileWithSchemaOptionalDateComparatorAllowsNotNullDateFallback(t *testing.T) {
	assert := assert.New(t)
	schema := dateFieldSchemaWithRequired("otherDate")

	jp, err := CompileWithSchema("not_null(createdDate, '2026-03-01') < otherDate", schema)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestCompileWithSchemaOptionalDateComparatorAllowsOrDateFallback(t *testing.T) {
	assert := assert.New(t)
	schema := dateFieldSchemaWithRequired("otherDate")

	jp, err := CompileWithSchema("(createdDate || '2026-03-01') < otherDate", schema)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestCompileWithSchemaOptionalDateComparatorAllowsLiteralFirstOrDateFallback(t *testing.T) {
	assert := assert.New(t)
	schema := dateFieldSchemaWithRequired("otherDate")

	jp, err := CompileWithSchema("('2026-03-01' || createdDate) < otherDate", schema)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestCompileWithSchemaOptionalDateComparatorAllowsLiteralFirstNotNullFallback(t *testing.T) {
	assert := assert.New(t)
	schema := dateFieldSchemaWithRequired("otherDate")

	jp, err := CompileWithSchema("otherDate > not_null('2026-03-01', createdDate)", schema)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestCompileWithCompiledSchemaOptionalDateComparatorAllowsLiteralFirstOrDateFallback(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(dateFieldSchemaWithRequired("otherDate"))
	assert.NoError(err)

	jp, err := CompileWithCompiledSchema("('2026-03-01' || createdDate) < otherDate", cs)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestCompileWithCompiledSchemaDateComparators(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(schemaWithDateFields())
	assert.NoError(err)

	jp, err := CompileWithCompiledSchema("createdDate >= otherDate", cs)
	assert.NoError(err)
	assert.NotNil(jp)

	_, err = CompileWithCompiledSchema("createdDate >= status", cs)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrInvalidComparator, staticErr.Code)
}

func TestCompileWithCompiledSchemaOptionalDateComparatorsRequireGuard(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(optionalSchemaWithDateFields())
	assert.NoError(err)

	_, err = CompileWithCompiledSchema("createdDate >= otherDate", cs)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsafeOptionalArg, staticErr.Code)

	jp, err := CompileWithCompiledSchema("createdDate && createdDate >= '2026-03-01'", cs)
	assert.NoError(err)
	assert.NotNil(jp)

	jp, err = CompileWithCompiledSchema("not_null(createdDate, '2026-03-01') < not_null(otherDate, '2026-03-02')", cs)
	assert.NoError(err)
	assert.NotNil(jp)
}

func TestInferTypeWithCompiledSchemaDateComparator(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(schemaWithDateFields())
	assert.NoError(err)

	inferred, err := InferTypeWithCompiledSchema("createdDate >= '2026-03-01'", cs)
	assert.NoError(err)
	if assert.NotNil(inferred) {
		assert.True(inferred.IsBoolean())
	}
}

func TestInferTypeWithCompiledSchemaOptionalDateComparatorFails(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(optionalSchemaWithDateFields())
	assert.NoError(err)

	inferred, err := InferTypeWithCompiledSchema("createdDate >= '2026-03-01'", cs)
	assert.Error(err)
	assert.Nil(inferred)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsafeOptionalArg, staticErr.Code)
}
