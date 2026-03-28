package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtectsWhenTrueWithoutSchemaCompilation(t *testing.T) {
	a := assert.New(t)
	jp, err := Compile("a && b")
	a.NoError(err)
	if !a.NotNil(jp) {
		return
	}
	a.Nil(jp.GuardsWhenTrue())
	a.False(jp.ProtectsWhenTrue("a"))
	a.False(jp.ProtectsWhenTrue("b"))
}

func TestGuardAnalysisOptionsDefaultEnabled(t *testing.T) {
	a := assert.New(t)
	jp, err := CompileWithSchemaOptions("a && b", guardFlatSchema(false), &SchemaCompileOptions{})
	a.NoError(err)
	if !a.NotNil(jp) {
		return
	}
	a.NotNil(jp.GuardsWhenTrue())
	a.True(jp.ProtectsWhenTrue("a"))
	a.True(jp.ProtectsWhenTrue("b"))
}

func TestGuardAnalysisCanBeDisabled(t *testing.T) {
	a := assert.New(t)
	schema := guardFlatSchema(false)
	cs, err := CompileSchema(schema)
	a.NoError(err)
	if !a.NotNil(cs) {
		return
	}
	options := &SchemaCompileOptions{DisableGuardAnalysis: true}
	modes := []struct {
		name    string
		compile func(string) (*JMESPath, error)
	}{
		{
			name: "CompileWithSchemaOptions",
			compile: func(expression string) (*JMESPath, error) {
				return CompileWithSchemaOptions(expression, schema, options)
			},
		},
		{
			name: "CompileWithCompiledSchemaOptions",
			compile: func(expression string) (*JMESPath, error) {
				return CompileWithCompiledSchemaOptions(expression, cs, options)
			},
		},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			subAssert := assert.New(t)
			jp, compileErr := mode.compile("a && b")
			subAssert.NoError(compileErr)
			if !subAssert.NotNil(jp) {
				return
			}
			subAssert.Nil(jp.GuardsWhenTrue())
			subAssert.False(jp.ProtectsWhenTrue("a"))
			subAssert.False(jp.ProtectsWhenTrue("b"))
		})
	}
}

func TestGuardAnalysisWhenTrue(t *testing.T) {
	tests := []struct {
		name         string
		schema       JSONSchema
		expression   string
		protected    []string
		notProtected []string
	}{
		{
			name:       "field",
			schema:     guardFlatSchema(false),
			expression: "a",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
				"a.b",
			},
		},
		{
			name:       "and",
			schema:     guardFlatSchema(false),
			expression: "a && b",
			protected:  []string{"a", "b"},
			notProtected: []string{
				"a.b",
			},
		},
		{
			name:       "or",
			schema:     guardFlatSchema(false),
			expression: "a || b",
			protected:  nil,
			notProtected: []string{
				"a",
				"b",
			},
		},
		{
			name:       "subexpression",
			schema:     guardNestedSchema(false),
			expression: "a.b",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "and_subexpression",
			schema:     guardNestedSchema(false),
			expression: "a && a.b",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "or_common_prefix",
			schema:     guardNestedOrSchema(),
			expression: "a.b || a.c",
			protected:  []string{"a"},
			notProtected: []string{
				"a.b",
				"a.c",
			},
		},
		{
			name:       "neq_null",
			schema:     guardFlatSchema(false),
			expression: "a != `null`",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "neq_null_nested_path_adds_prefixes",
			schema:     guardNestedSchema(false),
			expression: "a.b != `null`",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "eq_null",
			schema:     guardFlatSchema(false),
			expression: "a == `null`",
			protected:  nil,
			notProtected: []string{
				"a",
			},
		},
		{
			name:       "eq_non_null_literal",
			schema:     guardFlatStringSchema(false),
			expression: "a == 'x'",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "eq_non_null_literal_reversed",
			schema:     guardFlatStringSchema(false),
			expression: "'x' == a",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "eq_non_null_literal_nested_path_adds_prefixes",
			schema:     guardNestedSchema(false),
			expression: "a.b == 'a'",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "neq_non_null_literal_does_not_guard",
			schema:     guardFlatStringSchema(false),
			expression: "a != 'x'",
			protected:  nil,
			notProtected: []string{
				"a",
			},
		},
		{
			name:       "gt_number_literal_guards_path",
			schema:     guardFlatSchema(false),
			expression: "a > `0`",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "lt_number_literal_reversed_guards_path",
			schema:     guardFlatSchema(false),
			expression: "`0` < a",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "gte_nested_path_adds_prefixes",
			schema:     guardNestedNumberSchema(false),
			expression: "a.b >= `1`",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "lt_nested_path_adds_prefixes",
			schema:     guardNestedNumberSchema(false),
			expression: "a.b < `10`",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "lte_number_literal_guards_path",
			schema:     guardFlatSchema(false),
			expression: "a <= `10`",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "gt_two_paths_guards_both",
			schema:     guardFlatSchema(false),
			expression: "a > b",
			protected:  []string{"a", "b"},
			notProtected: []string{
				"a.b",
			},
		},
		{
			name:       "gte_two_nested_paths_guards_both_prefixes",
			schema:     guardDualNestedNumberSchema(),
			expression: "a.b >= c.d",
			protected:  []string{"a", "a.b", "c", "c.d"},
			notProtected: []string{
				"a.c",
				"c.a",
			},
		},
		{
			name:       "not_expression",
			schema:     guardFlatSchema(false),
			expression: "!a",
			protected:  nil,
			notProtected: []string{
				"a",
			},
		},
		{
			name:       "nested_optional_object",
			schema:     guardNestedSchema(false),
			expression: "a && a.b",
			protected:  []string{"a", "a.b"},
			notProtected: []string{
				"a.c",
			},
		},
		{
			name:       "closed_schema",
			schema:     guardKnownSchema(false),
			expression: "known",
			protected:  []string{"known"},
			notProtected: []string{
				"unknown",
			},
		},
		{
			name:       "open_schema",
			schema:     guardKnownSchema(true),
			expression: "known",
			protected:  []string{"known"},
			notProtected: []string{
				"unknown",
			},
		},
		{
			name:       "unsupported_function_expression",
			schema:     guardFlatSchema(false),
			expression: "abs(a)",
			protected:  nil,
			notProtected: []string{
				"a",
			},
		},
		{
			name:       "function_starts_with_guards_first_arg",
			schema:     guardFlatStringSchema(false),
			expression: "starts_with(a, 'x')",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_starts_with_guards_both_path_args",
			schema:     guardFlatStringSchema(false),
			expression: "starts_with(a, b)",
			protected:  []string{"a", "b"},
			notProtected: []string{
				"a.b",
			},
		},
		{
			name:       "function_ends_with_guards_first_arg",
			schema:     guardFlatStringSchema(false),
			expression: "ends_with(a, 'x')",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_contains_guards_first_arg",
			schema:     guardFlatStringSchema(false),
			expression: "contains(a, 'x')",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_contains_string_literal_search_guards_second_arg",
			schema:     guardFlatStringSchema(false),
			expression: "contains('abc', a)",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_contains_literal_array_guards_second_arg",
			schema:     guardFlatStringSchema(false),
			expression: "contains(['a', 'b'], a)",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_contains_literal_array_with_null_does_not_guard_second_arg",
			schema:     guardFlatStringSchema(false),
			expression: "contains([`null`, 'a'], a)",
			protected:  nil,
			notProtected: []string{
				"a",
				"b",
			},
		},
		{
			name:       "function_contains_path_args_guards_only_first_arg",
			schema:     guardFlatStringSchema(false),
			expression: "contains(a, b)",
			protected:  []string{"a"},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "function_contains_in_and_combines_guards",
			schema:     guardFlatStringSchema(false),
			expression: "a && contains(['x'], b)",
			protected:  []string{"a", "b"},
			notProtected: []string{
				"a.b",
			},
		},
		{
			name:       "function_in_and_keeps_left_guard",
			schema:     guardFlatSchema(false),
			expression: "a && abs(a)",
			protected: []string{
				"a",
			},
			notProtected: []string{
				"b",
			},
		},
		{
			name:       "unsupported_projection_expression",
			schema:     guardProjectionSchema(),
			expression: "items[].price",
			protected:  nil,
			notProtected: []string{
				"items",
				"items.price",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			jp, err := CompileWithSchema(tt.expression, tt.schema)
			a.NoError(err)
			if !a.NotNil(jp) {
				return
			}

			guards := jp.GuardsWhenTrue()
			if !a.NotNil(guards) {
				return
			}
			a.Equal(tt.protected, guards.ProtectedPaths())

			for _, path := range tt.protected {
				a.True(jp.ProtectsWhenTrue(path), path)
				a.True(guards.Protects(path), path)
			}
			for _, path := range tt.notProtected {
				a.False(jp.ProtectsWhenTrue(path), path)
				a.False(guards.Protects(path), path)
			}
		})
	}
}

func TestGuardAnalysisProtectedPathsReturnsCopy(t *testing.T) {
	a := assert.New(t)
	jp, err := CompileWithSchema("a && a.b", guardNestedSchema(false))
	a.NoError(err)
	if !a.NotNil(jp) {
		return
	}
	guards := jp.GuardsWhenTrue()
	if !a.NotNil(guards) {
		return
	}
	first := guards.ProtectedPaths()
	a.Equal([]string{"a", "a.b"}, first)
	first[0] = "mutated"
	second := guards.ProtectedPaths()
	a.Equal([]string{"a", "a.b"}, second)
}

func guardFlatSchema(open bool) JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{"type": "number"},
			"b": map[string]interface{}{"type": "number"},
		},
		"additionalProperties": open,
	}
}

func guardFlatStringSchema(open bool) JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{"type": "string"},
			"b": map[string]interface{}{"type": "string"},
		},
		"additionalProperties": open,
	}
}

func guardNestedSchema(open bool) JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"b": map[string]interface{}{"type": "string"},
				},
				"additionalProperties": open,
			},
		},
		"additionalProperties": false,
	}
}

func guardNestedNumberSchema(open bool) JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"b": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": open,
			},
		},
		"additionalProperties": false,
	}
}

func guardDualNestedNumberSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"b": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
			"c": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"d": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}

func guardNestedOrSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"a": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"b": map[string]interface{}{"type": "string"},
					"c": map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}

func guardKnownSchema(open bool) JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"known": map[string]interface{}{"type": "string"},
		},
		"additionalProperties": open,
	}
}

func guardProjectionSchema() JSONSchema {
	return JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"price": map[string]interface{}{"type": "number"},
					},
					"additionalProperties": false,
				},
			},
		},
		"additionalProperties": false,
	}
}
