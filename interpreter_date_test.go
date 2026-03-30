package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDateComparatorRuntime(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		schema     JSONSchema
		input      map[string]interface{}
		expected   interface{}
	}{
		{
			name:       "literal comparator matches newer date",
			expression: "createdDate >= '2026-03-01'",
			schema:     schemaWithDateFields(),
			input:      map[string]interface{}{"createdDate": "2026-03-05"},
			expected:   true,
		},
		{
			name:       "literal comparator rejects older date",
			expression: "createdDate >= '2026-03-01'",
			schema:     schemaWithDateFields(),
			input:      map[string]interface{}{"createdDate": "2026-02-27"},
			expected:   false,
		},
		{
			name:       "field comparator matches ordered dates",
			expression: "createdDate < otherDate",
			schema:     schemaWithDateFields(),
			input: map[string]interface{}{
				"createdDate": "2026-03-01",
				"otherDate":   "2026-03-02",
			},
			expected: true,
		},
		{
			name:       "field comparator rejects reversed dates",
			expression: "createdDate < otherDate",
			schema:     schemaWithDateFields(),
			input: map[string]interface{}{
				"createdDate": "2026-03-03",
				"otherDate":   "2026-03-02",
			},
			expected: false,
		},
		{
			name:       "not_null fallback uses literal when field is missing",
			expression: "not_null(createdDate, '2026-03-01') < otherDate",
			schema:     dateFieldSchemaWithRequired("otherDate"),
			input: map[string]interface{}{
				"otherDate": "2026-03-02",
			},
			expected: true,
		},
		{
			name:       "not_null fallback uses field value when present",
			expression: "not_null(createdDate, '2026-03-01') < otherDate",
			schema:     dateFieldSchemaWithRequired("otherDate"),
			input: map[string]interface{}{
				"createdDate": "2026-03-03",
				"otherDate":   "2026-03-02",
			},
			expected: false,
		},
		{
			name:       "literal-first or fallback short-circuits to literal",
			expression: "('2026-03-01' || createdDate) < otherDate",
			schema:     dateFieldSchemaWithRequired("otherDate"),
			input: map[string]interface{}{
				"otherDate": "2026-03-02",
			},
			expected: true,
		},
		{
			name:       "literal-first or fallback ignores later field value",
			expression: "('2026-03-01' || createdDate) < otherDate",
			schema:     dateFieldSchemaWithRequired("otherDate"),
			input: map[string]interface{}{
				"createdDate": "2026-03-03",
				"otherDate":   "2026-03-02",
			},
			expected: true,
		},
		{
			name:       "filter keeps only matching dates",
			expression: "items[?createdDate >= '2026-03-01']",
			schema:     benchmarkLiteralDateArraySchema(),
			input: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"createdDate": "2026-02-27"},
					map[string]interface{}{"createdDate": "2026-03-05"},
				},
			},
			expected: []interface{}{
				map[string]interface{}{"createdDate": "2026-03-05"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range dateCompileModes(t, tt.schema) {
				mode := mode
				t.Run(mode.name, func(t *testing.T) {
					jp, err := mode.compile(tt.expression)
					assert.NoError(t, err)
					if !assert.NotNil(t, jp) {
						return
					}

					result, err := jp.Search(tt.input)
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				})
			}
		})
	}
}

func TestDateComparatorPrecomputesLiteralDates(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		left       bool
		right      bool
	}{
		{
			name:       "field compared to literal on right",
			expression: "createdDate >= '2026-03-01'",
			left:       false,
			right:      true,
		},
		{
			name:       "literal compared to field on left",
			expression: "'2026-03-01' <= createdDate",
			left:       true,
			right:      false,
		},
		{
			name:       "field to field comparator has no literals",
			expression: "createdDate < otherDate",
			left:       false,
			right:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			jp, err := CompileWithSchema(tt.expression, schemaWithDateFields())
			assert.NoError(t, err)
			if !assert.NotNil(t, jp) {
				return
			}

			plan := jp.intr.comparatorPlan(jp.ast)
			assert.Equal(t, orderedValueKindDate, plan.kind, tt.expression)
			assert.Equal(t, tt.left, plan.leftDateLiteral != "", tt.expression)
			assert.Equal(t, tt.right, plan.rightDateLiteral != "", tt.expression)
		})
	}
}

func TestDateComparatorRuntimeRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		input      map[string]interface{}
	}{
		{
			name:       "rejects invalid left date string",
			expression: "createdDate >= otherDate",
			input: map[string]interface{}{
				"createdDate": "draft",
				"otherDate":   "2026-03-02",
			},
		},
		{
			name:       "rejects nil right date",
			expression: "createdDate >= otherDate",
			input: map[string]interface{}{
				"createdDate": "2026-03-01",
				"otherDate":   nil,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range dateCompileModes(t, schemaWithDateFields()) {
				mode := mode
				t.Run(mode.name, func(t *testing.T) {
					jp, err := mode.compile(tt.expression)
					assert.NoError(t, err)
					if !assert.NotNil(t, jp) {
						return
					}

					_, err = jp.Search(tt.input)
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "invalid-type")
				})
			}
		})
	}
}
