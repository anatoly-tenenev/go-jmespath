package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompileWithCompiledSchemaReuse(t *testing.T) {
	assert := assert.New(t)
	schema := compileTestSchema()
	cs, err := CompileSchema(schema)
	assert.NoError(err)
	assert.NotNil(cs)

	expressions := []string{
		"foo.bar.baz[0]",
		"items[].price",
		"length(name)",
	}
	for _, expression := range expressions {
		jp, compileErr := CompileWithCompiledSchema(expression, cs)
		assert.NoError(compileErr, expression)
		assert.NotNil(jp, expression)
	}
}

func TestCompileWithCompiledSchemaNil(t *testing.T) {
	assert := assert.New(t)
	_, err := CompileWithCompiledSchema("foo", nil)
	assert.Error(err)
	var staticErr *StaticError
	assert.ErrorAs(err, &staticErr)
	assert.Equal(staticErrUnsupportedSchema, staticErr.Code)
}

func TestMustCompileWithSchemaPanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	schema := compileTestSchema()
	MustCompileWithSchema("foo.unknown", schema)
}
