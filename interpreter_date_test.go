package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaAwareDateComparatorRuntime(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("createdDate >= '2026-03-01'", schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{"createdDate": "2026-03-05"})
	assert.NoError(err)
	assert.Equal(true, result)

	result, err = jp.Search(map[string]interface{}{"createdDate": "2026-02-27"})
	assert.NoError(err)
	assert.Equal(false, result)
}

func TestSchemaAwareDateFieldToFieldComparatorRuntime(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("createdDate < otherDate", schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{
		"createdDate": "2026-03-01",
		"otherDate":   "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(true, result)

	result, err = jp.Search(map[string]interface{}{
		"createdDate": "2026-03-03",
		"otherDate":   "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(false, result)
}

func TestSchemaAwareDateComparatorRuntimeWithNotNullDateFallback(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("not_null(createdDate, '2026-03-01') < otherDate", dateFieldSchemaWithRequired("otherDate"))
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{
		"otherDate": "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(true, result)

	result, err = jp.Search(map[string]interface{}{
		"createdDate": "2026-03-03",
		"otherDate":   "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(false, result)
}

func TestSchemaAwareDateComparatorRuntimeWithLiteralFirstOrDateFallback(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("('2026-03-01' || createdDate) < otherDate", dateFieldSchemaWithRequired("otherDate"))
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{
		"otherDate": "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(true, result)

	result, err = jp.Search(map[string]interface{}{
		"createdDate": "2026-03-03",
		"otherDate":   "2026-03-02",
	})
	assert.NoError(err)
	assert.Equal(true, result)
}

func TestSchemaAwareDateComparatorRuntimeInFilter(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("items[?createdDate >= '2026-03-01']", benchmarkLiteralDateArraySchema())
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"createdDate": "2026-02-27"},
			map[string]interface{}{"createdDate": "2026-03-05"},
		},
	})
	assert.NoError(err)
	assert.Equal([]interface{}{
		map[string]interface{}{"createdDate": "2026-03-05"},
	}, result)
}

func TestSchemaAwareDateComparatorPrecomputesLiteralDates(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		expression string
		left       bool
		right      bool
	}{
		{expression: "createdDate >= '2026-03-01'", left: false, right: true},
		{expression: "'2026-03-01' <= createdDate", left: true, right: false},
		{expression: "createdDate < otherDate", left: false, right: false},
	}

	for _, tt := range tests {
		jp, err := CompileWithSchema(tt.expression, schemaWithDateFields())
		assert.NoError(err, tt.expression)
		if !assert.NotNil(jp, tt.expression) {
			continue
		}

		plan := jp.intr.comparatorPlan(jp.ast)
		assert.Equal(orderedValueKindDate, plan.kind, tt.expression)
		assert.Equal(tt.left, plan.leftDateLiteral != "", tt.expression)
		assert.Equal(tt.right, plan.rightDateLiteral != "", tt.expression)
	}
}

func TestSchemaAwareDateComparatorRuntimeRejectsInvalidValues(t *testing.T) {
	assert := assert.New(t)
	jp, err := CompileWithSchema("createdDate >= otherDate", schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	_, err = jp.Search(map[string]interface{}{
		"createdDate": "draft",
		"otherDate":   "2026-03-02",
	})
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-type")

	_, err = jp.Search(map[string]interface{}{
		"createdDate": "2026-03-01",
		"otherDate":   nil,
	})
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-type")
}

func TestCompiledSchemaDateComparatorRuntime(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(cs) {
		return
	}

	jp, err := CompileWithCompiledSchema("createdDate >= '2026-03-01'", cs)
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	result, err := jp.Search(map[string]interface{}{"createdDate": "2026-03-05"})
	assert.NoError(err)
	assert.Equal(true, result)

	result, err = jp.Search(map[string]interface{}{"createdDate": "2026-02-27"})
	assert.NoError(err)
	assert.Equal(false, result)
}

func TestCompiledSchemaDateComparatorRuntimeRejectsInvalidValues(t *testing.T) {
	assert := assert.New(t)
	cs, err := CompileSchema(schemaWithDateFields())
	assert.NoError(err)
	if !assert.NotNil(cs) {
		return
	}

	jp, err := CompileWithCompiledSchema("createdDate >= otherDate", cs)
	assert.NoError(err)
	if !assert.NotNil(jp) {
		return
	}

	_, err = jp.Search(map[string]interface{}{
		"createdDate": "draft",
		"otherDate":   "2026-03-02",
	})
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-type")

	_, err = jp.Search(map[string]interface{}{
		"createdDate": "2026-03-01",
		"otherDate":   nil,
	})
	assert.Error(err)
	assert.Contains(err.Error(), "invalid-type")
}
