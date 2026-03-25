package jmespath

import "strconv"

// JMESPath is the representation of a compiled JMES path query. A JMESPath is
// safe for concurrent use by multiple goroutines.
type JMESPath struct {
	ast  ASTNode
	intr *treeInterpreter
}

// Compile parses a JMESPath expression and returns, if successful, a JMESPath
// object that can be used to match against data.
func Compile(expression string) (*JMESPath, error) {
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	jmespath := &JMESPath{ast: ast, intr: newInterpreter()}
	return jmespath, nil
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled
// JMESPaths.
func MustCompile(expression string) *JMESPath {
	jmespath, err := Compile(expression)
	if err != nil {
		panic(`jmespath: Compile(` + strconv.Quote(expression) + `): ` + err.Error())
	}
	return jmespath
}

// CompileWithSchema parses and statically validates a JMESPath expression
// against the provided JSON schema.
func CompileWithSchema(expression string, schema JSONSchema) (*JMESPath, error) {
	cs, err := CompileSchema(schema)
	if err != nil {
		return nil, err
	}
	return CompileWithCompiledSchema(expression, cs)
}

// MustCompileWithSchema is like CompileWithSchema but panics on error.
func MustCompileWithSchema(expression string, schema JSONSchema) *JMESPath {
	jmespath, err := CompileWithSchema(expression, schema)
	if err != nil {
		panic(`jmespath: CompileWithSchema(` + strconv.Quote(expression) + `): ` + err.Error())
	}
	return jmespath
}

// CompileWithCompiledSchema parses and statically validates a JMESPath
// expression against a precompiled schema.
func CompileWithCompiledSchema(expression string, cs *CompiledSchema) (*JMESPath, error) {
	if cs == nil || cs.root == nil {
		return nil, unsupportedSchemaError("$", "compiled schema is nil")
	}
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	if _, err := analyzeExpressionAgainstSchema(expression, ast, cs); err != nil {
		return nil, err
	}
	return &JMESPath{ast: ast, intr: newInterpreter()}, nil
}

// InferTypeWithSchema parses and statically validates a JMESPath expression
// against the provided schema and returns the inferred result type.
func InferTypeWithSchema(expression string, schema JSONSchema) (*InferredType, error) {
	cs, err := CompileSchema(schema)
	if err != nil {
		return nil, err
	}
	return InferTypeWithCompiledSchema(expression, cs)
}

// InferTypeWithCompiledSchema parses and statically validates a JMESPath
// expression against a precompiled schema and returns the inferred result type.
// This is the preferred fast path for repeated queries against the same schema.
func InferTypeWithCompiledSchema(expression string, cs *CompiledSchema) (*InferredType, error) {
	if cs == nil || cs.root == nil {
		return nil, unsupportedSchemaError("$", "compiled schema is nil")
	}
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	inferredStaticType, err := analyzeExpressionAgainstSchema(expression, ast, cs)
	if err != nil {
		return nil, err
	}
	return inferredTypeFromStatic(inferredStaticType), nil
}

// Search evaluates a JMESPath expression against input data and returns the result.
func (jp *JMESPath) Search(data interface{}) (interface{}, error) {
	return jp.intr.Execute(jp.ast, data)
}

// Search evaluates a JMESPath expression against input data and returns the result.
func Search(expression string, data interface{}) (interface{}, error) {
	intr := newInterpreter()
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	return intr.Execute(ast, data)
}
