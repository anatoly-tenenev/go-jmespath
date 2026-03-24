package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompileWithSchemaSuccessCases(t *testing.T) {
	assert := assert.New(t)
	schema := compileTestSchema()
	expressions := []string{
		"foo.bar.baz[2]",
		"items[].price",
		"length(name)",
		"max_by(items, &price)",
		"length(items[3].name)",
		"contains(items[3].name, 'foo')",
	}

	for _, expression := range expressions {
		jp, err := CompileWithSchema(expression, schema)
		assert.NoError(err, expression)
		assert.NotNil(jp, expression)
	}
}

func TestCompileWithSchemaErrors(t *testing.T) {
	assert := assert.New(t)
	schema := compileTestSchema()
	tests := []struct {
		expression string
		code       string
		offset     int
	}{
		{expression: "foo.bra", code: staticErrUnknownProperty, offset: 4},
		{expression: "name.foo", code: staticErrInvalidFieldTarget, offset: 5},
		{expression: "name[0]", code: staticErrInvalidIndexTarget, offset: 4},
		{expression: "name[]", code: staticErrInvalidProjection, offset: 4},
		{expression: "items[0].price > name", code: staticErrInvalidComparator, offset: 15},
		{expression: "abs(name)", code: staticErrInvalidFuncArgType, offset: 4},
		{expression: "unknown(name)", code: staticErrUnknownFunction, offset: 0},
		{expression: "length(name, name)", code: staticErrInvalidFuncArity, offset: 0},
	}

	for _, tt := range tests {
		_, err := CompileWithSchema(tt.expression, schema)
		assert.Error(err, tt.expression)
		var staticErr *StaticError
		assert.ErrorAs(err, &staticErr, tt.expression)
		assert.Equal(tt.code, staticErr.Code, tt.expression)
		assert.Equal(tt.offset, staticErr.Offset, tt.expression)
	}
}

func TestCompileWithSchemaAdditionalPropertiesModes(t *testing.T) {
	assert := assert.New(t)

	openSchema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"known": map[string]interface{}{"type": "string"},
		},
		"additionalProperties": true,
	}
	_, err := CompileWithSchema("unknown", openSchema)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnverifiableProperty, staticErr.Code)
	assert.Equal(0, staticErr.Offset)

	typedOpenSchema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"known": map[string]interface{}{"type": "string"},
		},
		"additionalProperties": map[string]interface{}{"type": "number"},
	}
	_, err = CompileWithSchema("unknown", typedOpenSchema)
	assert.Error(err)
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnverifiableProperty, staticErr.Code)
	assert.Equal(0, staticErr.Offset)

	closedSchema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"known": map[string]interface{}{"type": "string"},
		},
		"additionalProperties": false,
	}
	_, err = CompileWithSchema("unknwon", closedSchema)
	assert.Error(err)
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnknownProperty, staticErr.Code)
}

func TestCompileWithSchemaOptionalPropertyCompiles(t *testing.T) {
	assert := assert.New(t)
	schema := JSONSchema{
		"type": "object",
		"properties": map[string]interface{}{
			"optional": map[string]interface{}{"type": "string"},
		},
		"required":             []interface{}{},
		"additionalProperties": false,
	}

	_, err := CompileWithSchema("optional", schema)
	assert.NoError(err)
}
