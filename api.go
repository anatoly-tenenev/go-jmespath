package jmespath

import "strconv"

// SchemaCompileOptions controls optional schema-aware compile behaviors.
type SchemaCompileOptions struct {
	// DisableGuardAnalysis disables guard analysis for schema-aware compilation.
	// By default guard analysis is enabled.
	DisableGuardAnalysis bool
}

// JMESPath is the representation of a compiled JMES path query. A JMESPath is
// safe for concurrent use by multiple goroutines.
type JMESPath struct {
	ast            ASTNode
	intr           *treeInterpreter
	guardsWhenTrue *GuardAnalysis
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
	return CompileWithSchemaOptions(expression, schema, nil)
}

// CompileWithSchemaOptions parses and statically validates a JMESPath expression
// against the provided JSON schema with optional schema-aware compile settings.
func CompileWithSchemaOptions(expression string, schema JSONSchema, options *SchemaCompileOptions) (*JMESPath, error) {
	cs, err := CompileSchema(schema)
	if err != nil {
		return nil, err
	}
	return CompileWithCompiledSchemaOptions(expression, cs, options)
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
	return CompileWithCompiledSchemaOptions(expression, cs, nil)
}

// CompileWithCompiledSchemaOptions parses and statically validates a JMESPath
// expression against a precompiled schema with optional schema-aware compile settings.
func CompileWithCompiledSchemaOptions(expression string, cs *CompiledSchema, options *SchemaCompileOptions) (*JMESPath, error) {
	if cs == nil || cs.root == nil {
		return nil, unsupportedSchemaError("$", "compiled schema is nil")
	}
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	analysis, err := analyzeExpressionAgainstSchema(expression, ast, cs)
	if err != nil {
		return nil, err
	}
	var guardsWhenTrue *GuardAnalysis
	if options == nil || !options.DisableGuardAnalysis {
		guardsWhenTrue = analyzeGuardsWhenTrue(ast)
	}
	return &JMESPath{
		ast:            ast,
		// The interpreter consumes compile-time comparator plans to avoid repeating
		// date-format checks and literal parsing work during Search.
		intr:           newInterpreterWithComparatorPlans(analysis.comparatorPlans),
		guardsWhenTrue: guardsWhenTrue,
	}, nil
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
	analysis, err := analyzeExpressionAgainstSchema(expression, ast, cs)
	if err != nil {
		return nil, err
	}
	return inferredTypeFromStatic(analysis.resultType), nil
}

// Search evaluates a JMESPath expression against input data and returns the result.
func (jp *JMESPath) Search(data interface{}) (interface{}, error) {
	return jp.intr.Execute(jp.ast, data)
}

// ProtectsWhenTrue reports whether the given path is guaranteed to be
// non-missing and non-null when the expression evaluates to true.
//
// This information is available only for expressions compiled via
// CompileWithSchema or CompileWithCompiledSchema.
func (jp *JMESPath) ProtectsWhenTrue(path string) bool {
	if jp == nil || jp.guardsWhenTrue == nil {
		return false
	}
	return jp.guardsWhenTrue.Protects(path)
}

// GuardsWhenTrue returns schema-aware guard analysis for this expression.
//
// For expressions compiled without schema-aware APIs this returns nil.
func (jp *JMESPath) GuardsWhenTrue() *GuardAnalysis {
	if jp == nil {
		return nil
	}
	return jp.guardsWhenTrue
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
