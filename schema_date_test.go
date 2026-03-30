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

type dateCompileExpectation struct {
	expression string
	code       string
}

func TestCompileSchemaSupportsStringDateFormat(t *testing.T) {
	compiled, err := CompileSchema(schemaWithDateFields())
	assert.NoError(t, err)
	if !assert.NotNil(t, compiled) || !assert.NotNil(t, compiled.root) || !assert.NotNil(t, compiled.staticRoot) {
		return
	}

	createdDate := compiled.root.properties["createdDate"]
	assert.NotNil(t, createdDate)
	assert.Equal(t, stringFormatDate, createdDate.stringFormat)

	staticCreatedDate := compiled.staticRoot.object.properties["createdDate"]
	assert.NotNil(t, staticCreatedDate)
	assert.Equal(t, stringFormatDate, staticCreatedDate.stringFormat)
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

func TestCompileSchemaDateFormatConstraints(t *testing.T) {
	tests := []struct {
		name            string
		schema          JSONSchema
		valid           bool
		expectedKeyword string
	}{
		{
			name: "valid date const",
			schema: JSONSchema{
				"format": "date",
				"const":  "2026-03-01",
			},
			valid: true,
		},
		{
			name: "valid date enum",
			schema: JSONSchema{
				"format": "date",
				"enum":   []interface{}{"2026-03-01", "2026-03-02"},
			},
			valid: true,
		},
		{
			name: "invalid date const",
			schema: JSONSchema{
				"type":   "string",
				"format": "date",
				"const":  "draft",
			},
			valid:           false,
			expectedKeyword: "valid date",
		},
		{
			name: "invalid date enum",
			schema: JSONSchema{
				"type":   "string",
				"format": "date",
				"enum":   []interface{}{"2026-03-01", "draft"},
			},
			valid:           false,
			expectedKeyword: "valid date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.valid {
				assertUnsupportedSchemaCompileError(t, tt.schema, "$", tt.expectedKeyword)
				return
			}

			compiled := compileSchemaForTest(t, tt.schema)
			if !assert.NotNil(t, compiled.root) || !assert.NotNil(t, compiled.staticRoot) {
				return
			}

			assert.Equal(t, schemaKindString, compiled.root.kind)
			assert.Equal(t, stringFormatDate, compiled.root.stringFormat)
			assert.Equal(t, stringFormatDate, compiled.staticRoot.stringFormat)
		})
	}
}

func TestSchemaCompileDateComparators(t *testing.T) {
	tests := []struct {
		name     string
		schema   JSONSchema
		success  []string
		failures []dateCompileExpectation
	}{
		{
			name:   "required date fields",
			schema: schemaWithDateFields(),
			success: []string{
				"createdDate >= '2026-03-01'",
				"createdDate < otherDate",
				"createdDate >= otherDate",
			},
			failures: []dateCompileExpectation{
				{expression: "createdDate >= 'draft'", code: staticErrInvalidComparator},
				{expression: "createdDate >= '2026-02-30'", code: staticErrInvalidComparator},
				{expression: "'2026-03-01' < '2026-03-02'", code: staticErrInvalidComparator},
				{expression: "createdDate >= status", code: staticErrInvalidComparator},
				{expression: "createdDate >= count", code: staticErrInvalidComparator},
				{expression: "createdDate >= `10`", code: staticErrInvalidComparator},
			},
		},
		{
			name:   "optional date fields stay comparable",
			schema: optionalSchemaWithDateFields(),
			success: []string{
				"createdDate >= '2026-03-01'",
				"createdDate < otherDate",
				"createdDate >= otherDate",
				"createdDate && createdDate >= '2026-03-01'",
				"createdDate != `null` && createdDate >= '2026-03-01'",
				"createdDate && otherDate && createdDate < otherDate",
				"not_null(createdDate, '2026-03-01') < not_null(otherDate, '2026-03-02')",
			},
			failures: []dateCompileExpectation{
				{expression: "createdDate >= 'draft'", code: staticErrInvalidComparator},
				{expression: "createdDate >= status", code: staticErrInvalidComparator},
				{expression: "createdDate >= `10`", code: staticErrInvalidComparator},
			},
		},
		{
			name:   "optional date fields allow fallbacks",
			schema: dateFieldSchemaWithRequired("otherDate"),
			success: []string{
				"not_null(createdDate, '2026-03-01') < otherDate",
				"(createdDate || '2026-03-01') < otherDate",
				"('2026-03-01' || createdDate) < otherDate",
				"otherDate > not_null('2026-03-01', createdDate)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range schemaCompileModes(t, tt.schema) {
				t.Run(mode.name, func(t *testing.T) {
					for _, expression := range tt.success {
						jp, err := mode.compile(expression)
						assert.NoError(t, err, expression)
						assert.NotNil(t, jp, expression)
					}

					for _, failure := range tt.failures {
						jp, err := mode.compile(failure.expression)
						assert.Nil(t, jp, failure.expression)
						assertStaticErrorCode(t, err, failure.code, failure.expression)
					}
				})
			}
		})
	}
}

func TestInferTypeWithCompiledSchemaDateComparators(t *testing.T) {
	tests := []struct {
		name       string
		schema     JSONSchema
		expression string
		mask       TypeMask
		code       string
	}{
		{
			name:       "required date comparator",
			schema:     schemaWithDateFields(),
			expression: "createdDate >= '2026-03-01'",
			mask:       TypeBoolean,
		},
		{
			name:       "optional date comparator to literal",
			schema:     optionalSchemaWithDateFields(),
			expression: "createdDate >= '2026-03-01'",
			mask:       TypeBoolean | TypeNull,
		},
		{
			name:       "optional date comparator to date field",
			schema:     optionalSchemaWithDateFields(),
			expression: "createdDate >= otherDate",
			mask:       TypeBoolean | TypeNull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferred, err := InferTypeWithCompiledSchema(tt.expression, compileSchemaForTest(t, tt.schema))
			if tt.code == "" {
				assert.NoError(t, err)
				if assert.NotNil(t, inferred) {
					assert.Equal(t, tt.mask, inferred.Mask)
				}
				return
			}

			assert.Nil(t, inferred)
			assertStaticErrorCode(t, err, tt.code, tt.expression)
		})
	}
}
