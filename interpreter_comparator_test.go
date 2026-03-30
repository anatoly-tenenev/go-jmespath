package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderedNumberComparatorRuntimeReturnsNullForNullableValues(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		expression string
	}{
		{
			name:       "missing field returns null",
			expression: "optional_number > `4`",
			input:      map[string]interface{}{},
		},
		{
			name:       "null field returns null",
			expression: "optional_number > `4`",
			input:      map[string]interface{}{"optional_number": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range schemaCompileModes(t, functionNullableSafetySchema()) {
				t.Run(mode.name, func(t *testing.T) {
					jp, err := mode.compile(tt.expression)
					assert.NoError(t, err)
					if !assert.NotNil(t, jp) {
						return
					}

					result, err := jp.Search(tt.input)
					assert.NoError(t, err)
					assert.Nil(t, result)
				})
			}
		})
	}
}
