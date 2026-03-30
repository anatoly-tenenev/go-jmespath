package jmespath

import "fmt"

type staticTypeMask uint8

const (
	staticMaskObject staticTypeMask = 1 << iota
	staticMaskArray
	staticMaskString
	staticMaskNumber
	staticMaskBoolean
	staticMaskNull
)

const staticMaskAny = staticMaskObject | staticMaskArray | staticMaskString | staticMaskNumber | staticMaskBoolean | staticMaskNull

type staticType struct {
	mask         staticTypeMask
	object       *staticObjectType
	array        *staticArrayType
	constValue   *scalarLiteral
	enumValues   *scalarLiteralSet
	stringFormat stringFormat
	formatSource stringFormatSource
}

type stringFormatSource uint8

const (
	stringFormatSourceNone stringFormatSource = 0
	// Only schema-derived formats may enable special comparator semantics.
	stringFormatSourceSchema stringFormatSource = 1 << iota
	// Literals keep their parsed format only to preserve it through helpers like
	// not_null(dateField, "2026-03-01"), not to allow literal-only date compares.
	stringFormatSourceLiteral
)

type staticObjectType struct {
	properties       map[string]*staticType
	required         map[string]struct{}
	additionalMode   additionalPropertiesMode
	additionalSchema *staticType
}

type staticArrayType struct {
	items *staticType
}

type orderedValueKind uint8

const (
	orderedValueKindUnknown orderedValueKind = iota
	orderedValueKindNumber
	orderedValueKindDate
)

type comparatorPlan struct {
	kind orderedValueKind
	// Schema-aware compilation prevalidates date literals once so Search avoids
	// reparsing the same YYYY-MM-DD constant for every evaluation.
	leftDateLiteral  string
	rightDateLiteral string
}

type staticTruthiness uint8

const (
	staticTruthinessUnknown staticTruthiness = iota
	staticTruthinessAlwaysFalse
	staticTruthinessAlwaysTrue
)

var (
	staticAnyTypeValue     = &staticType{mask: staticMaskAny}
	staticStringTypeValue  = &staticType{mask: staticMaskString}
	staticNumberTypeValue  = &staticType{mask: staticMaskNumber}
	staticBooleanTypeValue = &staticType{mask: staticMaskBoolean}
	staticNullTypeValue    = &staticType{mask: staticMaskNull}
	compileFunctionTable   = newFunctionCaller().functionTable
)

func staticFromSchema(root *schemaNode) *staticType {
	cache := make(map[*schemaNode]*staticType)
	return staticFromSchemaNode(root, cache)
}

func staticFromSchemaNode(node *schemaNode, cache map[*schemaNode]*staticType) *staticType {
	if node == nil {
		return staticAnyTypeValue
	}
	if cached, exists := cache[node]; exists {
		return cached
	}
	current := &staticType{mask: schemaKindMask(node.kind)}
	cache[node] = current
	current.constValue = node.constValue
	current.enumValues = node.enumValues
	current.stringFormat = node.stringFormat
	if node.stringFormat != stringFormatNone {
		current.formatSource = stringFormatSourceSchema
	}
	switch node.kind {
	case schemaKindObject:
		obj := &staticObjectType{
			properties:       nil,
			required:         node.required,
			additionalMode:   node.additionalPropertiesMode,
			additionalSchema: nil,
		}
		if len(node.properties) > 0 {
			obj.properties = make(map[string]*staticType, len(node.properties))
			for key, value := range node.properties {
				obj.properties[key] = staticFromSchemaNode(value, cache)
			}
		}
		if node.additionalPropertiesSchema != nil {
			obj.additionalSchema = staticFromSchemaNode(node.additionalPropertiesSchema, cache)
		}
		current.object = obj
	case schemaKindArray:
		current.array = &staticArrayType{items: staticFromSchemaNode(node.items, cache)}
	}
	return current
}

func schemaKindMask(kind schemaKind) staticTypeMask {
	switch kind {
	case schemaKindObject:
		return staticMaskObject
	case schemaKindArray:
		return staticMaskArray
	case schemaKindString:
		return staticMaskString
	case schemaKindNumber:
		return staticMaskNumber
	case schemaKindBoolean:
		return staticMaskBoolean
	case schemaKindNull:
		return staticMaskNull
	default:
		return staticMaskAny
	}
}

type schemaAnalyzer struct {
	expression      string
	nonNullPaths    map[string]struct{}
	comparatorPlans map[int]comparatorPlan
}

type schemaAnalysisResult struct {
	resultType      *staticType
	comparatorPlans map[int]comparatorPlan
}

func analyzeExpressionAgainstSchema(expression string, ast ASTNode, cs *CompiledSchema) (*schemaAnalysisResult, error) {
	if cs == nil || cs.root == nil {
		return nil, unsupportedSchemaError("$", "compiled schema is nil")
	}
	rootType := cs.staticRoot
	if rootType == nil {
		rootType = staticFromSchema(cs.root)
	}
	analyzer := &schemaAnalyzer{
		expression:      expression,
		comparatorPlans: make(map[int]comparatorPlan),
	}
	result, err := analyzer.analyze(ast, rootType)
	if err != nil {
		return nil, err
	}
	return &schemaAnalysisResult{
		resultType:      normalizeStaticType(result),
		comparatorPlans: analyzer.comparatorPlans,
	}, nil
}

func (a *schemaAnalyzer) analyze(node ASTNode, input *staticType) (*staticType, error) {
	var (
		result *staticType
		err    error
	)
	switch node.nodeType {
	case ASTEmpty, ASTCurrentNode, ASTIdentity:
		result = normalizeStaticType(input)
	case ASTField:
		result, err = a.analyzeField(node, input)
	case ASTSubexpression, ASTPipe:
		result, err = a.analyzeSequential(node, input)
	case ASTIndexExpression:
		result, err = a.analyzeIndexExpression(node, input)
	case ASTProjection:
		result, err = a.analyzeProjection(node, input)
	case ASTFilterProjection:
		result, err = a.analyzeFilterProjection(node, input)
	case ASTFlatten:
		result, err = a.analyzeFlatten(node, input)
	case ASTValueProjection:
		result, err = a.analyzeValueProjection(node, input)
	case ASTComparator:
		result, err = a.analyzeComparator(node, input)
	case ASTFunctionExpression:
		result, err = a.analyzeFunction(node, input)
	case ASTExpRef:
		if len(node.children) == 1 {
			_, err := a.analyze(node.children[0], input)
			if err != nil {
				return nil, err
			}
		}
		result = staticAnyTypeValue
	case ASTLiteral:
		result = staticTypeFromLiteral(node.value)
	case ASTMultiSelectList:
		result, err = a.analyzeMultiSelectList(node, input)
	case ASTMultiSelectHash:
		result, err = a.analyzeMultiSelectHash(node, input)
	case ASTKeyValPair:
		if len(node.children) == 0 {
			result = staticAnyTypeValue
			break
		}
		result, err = a.analyze(node.children[0], input)
	case ASTOrExpression, ASTAndExpression:
		result, err = a.analyzeLogical(node, input)
	case ASTNotExpression:
		if len(node.children) == 0 {
			result = staticBooleanTypeValue
			break
		}
		_, err := a.analyze(node.children[0], input)
		if err != nil {
			return nil, err
		}
		result = staticBooleanTypeValue
	case ASTSlice, ASTIndex:
		result = normalizeStaticType(input)
	default:
		result = staticAnyTypeValue
	}
	if err != nil {
		return nil, err
	}
	return a.applyNonNullNarrowing(node, result), nil
}

func (a *schemaAnalyzer) applyNonNullNarrowing(node ASTNode, typ *staticType) *staticType {
	if len(a.nonNullPaths) == 0 {
		return typ
	}
	path, ok := narrowablePath(node)
	if !ok {
		return typ
	}
	if _, exists := a.nonNullPaths[path]; !exists {
		return typ
	}
	return staticWithoutNull(typ)
}

func narrowablePath(node ASTNode) (string, bool) {
	switch node.nodeType {
	case ASTField, ASTSubexpression, ASTIndexExpression:
		path, ok := narrowableSchemaPath(node)
		if !ok || path == "" || path == "@" {
			return "", false
		}
		return path, true
	default:
		return "", false
	}
}

func (a *schemaAnalyzer) pushNonNullPathsFromGuardSet(paths guardPathSet) func() {
	if len(paths) == 0 {
		return func() {}
	}
	previous := a.nonNullPaths
	merged := make(map[string]struct{}, len(previous)+len(paths))
	for path := range previous {
		merged[path] = struct{}{}
	}
	for path := range paths {
		merged[path] = struct{}{}
	}
	a.nonNullPaths = merged
	return func() {
		a.nonNullPaths = previous
	}
}

func (a *schemaAnalyzer) analyzeSequential(node ASTNode, input *staticType) (*staticType, error) {
	current := normalizeStaticType(input)
	for _, child := range node.children {
		next, err := a.analyze(child, current)
		if err != nil {
			return nil, err
		}
		current = normalizeStaticType(next)
	}
	return current, nil
}

func (a *schemaAnalyzer) analyzeLogical(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 2 {
		return staticAnyTypeValue, nil
	}
	left, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	if node.nodeType == ASTAndExpression {
		guards := (&guardAnalyzer{}).analyzeWhenTrue(node.children[0])
		restoreNonNullPaths := a.pushNonNullPathsFromGuardSet(guards)
		defer restoreNonNullPaths()
	}
	right, err := a.analyze(node.children[1], input)
	if err != nil {
		return nil, err
	}
	if node.nodeType == ASTOrExpression {
		switch left.truthiness() {
		case staticTruthinessAlwaysTrue:
			return normalizeStaticType(left), nil
		case staticTruthinessAlwaysFalse:
			return normalizeStaticType(right), nil
		}
		// `a || b` returns `a` only when `a` is truthy. For unknown truthiness we
		// conservatively remove the null branch from the left side, because null
		// would force evaluation to fall through to `b`.
		leftWhenTruthy := staticWithoutNull(left)
		if staticTypeIsEmpty(leftWhenTruthy) {
			return normalizeStaticType(right), nil
		}
		return staticUnion(leftWhenTruthy, right), nil
	}
	return staticUnion(left, right), nil
}

func (a *schemaAnalyzer) analyzeField(node ASTNode, input *staticType) (*staticType, error) {
	target := normalizeStaticType(input)
	if !target.includes(staticMaskObject) {
		return nil, a.errorAt(node, staticErrInvalidFieldTarget, "field access requires object target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskObject) || targetWithoutNull.object == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove field target is object")
	}
	name, _ := node.value.(string)
	if targetWithoutNull.object.properties != nil {
		if value, exists := targetWithoutNull.object.properties[name]; exists {
			result := value
			if !targetWithoutNull.object.isRequired(name) {
				result = staticNullable(result)
			}
			if target.includes(staticMaskNull) {
				result = staticNullable(result)
			}
			return result, nil
		}
	}
	switch targetWithoutNull.object.additionalMode {
	case additionalPropertiesForbid:
		return nil, a.errorAt(node, staticErrUnknownProperty, fmt.Sprintf("unknown property %q", name))
	case additionalPropertiesAllowOpen, additionalPropertiesTyped:
		return nil, a.errorAt(node, staticErrUnverifiableProperty, fmt.Sprintf("property %q is not verifiable from schema", name))
	default:
		return nil, a.errorAt(node, staticErrUnverifiableProperty, fmt.Sprintf("property %q is not verifiable from schema", name))
	}
}

func (a *schemaAnalyzer) analyzeIndexExpression(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 2 {
		return staticAnyTypeValue, nil
	}
	target, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	target = normalizeStaticType(target)
	if !target.includes(staticMaskArray) {
		return nil, a.errorAt(node, staticErrInvalidIndexTarget, "index/slice requires array target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskArray) || targetWithoutNull.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove index target is array")
	}
	itemType := targetWithoutNull.array.itemType()
	if node.children[1].nodeType == ASTSlice {
		result := staticArrayOf(itemType)
		if target.includes(staticMaskNull) {
			result = staticNullable(result)
		}
		return result, nil
	}
	result := staticNullable(itemType)
	if target.includes(staticMaskNull) {
		result = staticNullable(result)
	}
	return result, nil
}

func (a *schemaAnalyzer) analyzeProjection(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 2 {
		return staticArrayOf(staticAnyTypeValue), nil
	}
	target, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	target = normalizeStaticType(target)
	if !target.includes(staticMaskArray) {
		return nil, a.errorAt(node, staticErrInvalidProjection, "projection requires array target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskArray) || targetWithoutNull.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove projection target is array")
	}
	result, err := a.analyze(node.children[1], targetWithoutNull.array.itemType())
	if err != nil {
		return nil, err
	}
	typedResult := staticArrayOf(result)
	if target.includes(staticMaskNull) {
		return staticNullable(typedResult), nil
	}
	return typedResult, nil
}

func (a *schemaAnalyzer) analyzeFilterProjection(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 3 {
		return staticArrayOf(staticAnyTypeValue), nil
	}
	target, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	target = normalizeStaticType(target)
	if !target.includes(staticMaskArray) {
		return nil, a.errorAt(node, staticErrInvalidProjection, "filter projection requires array target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskArray) || targetWithoutNull.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove filter projection target is array")
	}
	elementType := targetWithoutNull.array.itemType()
	_, err = a.analyze(node.children[2], elementType)
	if err != nil {
		return nil, err
	}
	result, err := a.analyze(node.children[1], elementType)
	if err != nil {
		return nil, err
	}
	typedResult := staticArrayOf(result)
	if target.includes(staticMaskNull) {
		return staticNullable(typedResult), nil
	}
	return typedResult, nil
}

func (a *schemaAnalyzer) analyzeFlatten(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 1 {
		return staticArrayOf(staticAnyTypeValue), nil
	}
	target, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	target = normalizeStaticType(target)
	if !target.includes(staticMaskArray) {
		return nil, a.errorAt(node, staticErrInvalidProjection, "flatten requires array target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskArray) || targetWithoutNull.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove flatten target is array")
	}
	itemType := targetWithoutNull.array.itemType()
	maybeNullTarget := target.includes(staticMaskNull)
	if itemType.isDefinite(staticMaskArray) && itemType.array != nil {
		result := staticArrayOf(itemType.array.itemType())
		if maybeNullTarget {
			return staticNullable(result), nil
		}
		return result, nil
	}
	if !itemType.includes(staticMaskArray) {
		result := staticArrayOf(itemType)
		if maybeNullTarget {
			return staticNullable(result), nil
		}
		return result, nil
	}
	merged := staticAnyTypeValue
	if itemType.array != nil {
		merged = itemType.array.itemType()
	}
	nonArrayMask := itemType.mask &^ staticMaskArray
	if nonArrayMask != 0 {
		merged = staticUnion(merged, &staticType{mask: nonArrayMask})
	}
	result := staticArrayOf(merged)
	if maybeNullTarget {
		return staticNullable(result), nil
	}
	return result, nil
}

func (a *schemaAnalyzer) analyzeValueProjection(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 2 {
		return staticArrayOf(staticAnyTypeValue), nil
	}
	target, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	target = normalizeStaticType(target)
	if !target.includes(staticMaskObject) {
		return nil, a.errorAt(node, staticErrInvalidProjection, "value projection requires object target")
	}
	targetWithoutNull := staticWithoutNull(target)
	if !targetWithoutNull.isDefinite(staticMaskObject) || targetWithoutNull.object == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove value projection target is object")
	}
	valuesType := targetWithoutNull.object.valuesType()
	result, err := a.analyze(node.children[1], valuesType)
	if err != nil {
		return nil, err
	}
	typedResult := staticArrayOf(result)
	if target.includes(staticMaskNull) {
		return staticNullable(typedResult), nil
	}
	return typedResult, nil
}

func (a *schemaAnalyzer) analyzeComparator(node ASTNode, input *staticType) (*staticType, error) {
	if len(node.children) != 2 {
		return staticBooleanTypeValue, nil
	}
	left, err := a.analyze(node.children[0], input)
	if err != nil {
		return nil, err
	}
	right, err := a.analyze(node.children[1], input)
	if err != nil {
		return nil, err
	}
	op, _ := node.value.(tokType)
	if op == tEQ || op == tNE {
		if err := a.validateComparatorLiteralMembership(node.children[0], node.children[1], left, right); err != nil {
			return nil, err
		}
		return staticBooleanTypeValue, nil
	}
	left = normalizeStaticType(left)
	right = normalizeStaticType(right)
	plan, err := a.validateOrderedComparator(node, node.children[0], node.children[1], left, right)
	if err != nil {
		return nil, err
	}
	// Persist the comparator plan on the AST offset so the interpreter can reuse
	// compile-time decisions on the runtime hot path.
	a.comparatorPlans[node.offset] = plan
	return orderedComparatorResultType(left, right), nil
}

func (a *schemaAnalyzer) validateOrderedComparator(node, leftNode, rightNode ASTNode, left, right *staticType) (comparatorPlan, error) {
	leftNonNull := staticWithoutNull(left)
	rightNonNull := staticWithoutNull(right)
	if leftNonNull.isDefinite(staticMaskNumber) && rightNonNull.isDefinite(staticMaskNumber) {
		return comparatorPlan{kind: orderedValueKindNumber}, nil
	}
	if leftNonNull.includes(staticMaskNumber) && rightNonNull.includes(staticMaskNumber) {
		return comparatorPlan{}, a.errorAt(node, staticErrUnverifiableType, "cannot prove comparator operands are numbers")
	}

	leftDateSchema := leftNonNull.hasStringFormat(stringFormatDate)
	rightDateSchema := rightNonNull.hasStringFormat(stringFormatDate)
	if leftDateSchema || rightDateSchema {
		leftMatches, leftLiteral := a.dateComparatorOperand(leftNode, leftNonNull, rightDateSchema)
		rightMatches, rightLiteral := a.dateComparatorOperand(rightNode, rightNonNull, leftDateSchema)
		if !leftMatches || !rightMatches {
			return comparatorPlan{}, a.errorAt(node, staticErrInvalidComparator, "comparator requires number operands or date operands")
		}
		return comparatorPlan{
			kind:             orderedValueKindDate,
			leftDateLiteral:  leftLiteral,
			rightDateLiteral: rightLiteral,
		}, nil
	}

	return comparatorPlan{}, a.errorAt(node, staticErrInvalidComparator, "comparator requires number operands or date operands")
}

func orderedComparatorResultType(left, right *staticType) *staticType {
	if normalizeStaticType(left).includes(staticMaskNull) || normalizeStaticType(right).includes(staticMaskNull) {
		return staticNullable(staticBooleanTypeValue)
	}
	return staticBooleanTypeValue
}

func (a *schemaAnalyzer) dateComparatorOperand(node ASTNode, typ *staticType, otherIsDateSchema bool) (bool, string) {
	// A schema date operand may be compared to another schema date or to a
	// prevalidated YYYY-MM-DD literal on the opposite side.
	if value, ok := dateLiteralValue(node); ok {
		return true, value
	}
	if typ.hasStringFormat(stringFormatDate) {
		return true, ""
	}
	if !otherIsDateSchema {
		return false, ""
	}
	if format, _, hasString := typ.nonNullStringFormat(); hasString && format == stringFormatDate {
		return true, ""
	}
	return false, ""
}

func (a *schemaAnalyzer) validateComparatorLiteralMembership(leftNode, rightNode ASTNode, leftType, rightType *staticType) error {
	if leftNode.nodeType == ASTLiteral {
		if err := a.validateLiteralAgainstConstraint(leftNode, rightNode, rightType, false); err != nil {
			return err
		}
	}
	if rightNode.nodeType == ASTLiteral {
		if err := a.validateLiteralAgainstConstraint(rightNode, leftNode, leftType, false); err != nil {
			return err
		}
	}
	return nil
}

func (a *schemaAnalyzer) validateContainsLiteralMembership(node ASTNode, argTypes []*staticType) error {
	if len(node.children) != 2 || len(argTypes) != 2 {
		return nil
	}
	literalNode := node.children[1]
	if literalNode.nodeType != ASTLiteral {
		return nil
	}
	arrayType := normalizeStaticType(argTypes[0])
	if !arrayType.isDefinite(staticMaskArray) || arrayType.array == nil {
		return nil
	}
	return a.validateLiteralAgainstConstraint(literalNode, node.children[0], arrayType.array.itemType(), true)
}

func (a *schemaAnalyzer) validateLiteralAgainstConstraint(literalNode, constrainedNode ASTNode, constrainedType *staticType, arrayItems bool) error {
	keyword, constrained := constrainedType.scalarConstraintKeyword()
	if !constrained {
		return nil
	}
	literalValue, ok := scalarLiteralFromInterface(literalNode.value)
	if ok && constrainedType.allowsScalarLiteral(literalValue) {
		return nil
	}
	literalText := formatLiteralForMessage(literalNode.value)
	path, hasPath := schemaFieldPath(constrainedNode)
	if arrayItems {
		if hasPath && path != "@" {
			return a.errorAt(literalNode, staticErrInvalidEnumValue, fmt.Sprintf("literal %s is not allowed by %s of array items in %s", literalText, keyword, path))
		}
		return a.errorAt(literalNode, staticErrInvalidEnumValue, fmt.Sprintf("literal %s is not allowed by %s of array items", literalText, keyword))
	}
	if hasPath && path != "@" {
		return a.errorAt(literalNode, staticErrInvalidEnumValue, fmt.Sprintf("literal %s is not allowed by %s of field %s", literalText, keyword, path))
	}
	return a.errorAt(literalNode, staticErrInvalidEnumValue, fmt.Sprintf("literal %s is not allowed by %s constraint", literalText, keyword))
}

func schemaFieldPath(node ASTNode) (string, bool) {
	switch node.nodeType {
	case ASTField:
		name, ok := node.value.(string)
		if !ok || name == "" {
			return "", false
		}
		return name, true
	case ASTSubexpression, ASTPipe:
		if len(node.children) != 2 {
			return "", false
		}
		leftPath, ok := schemaFieldPath(node.children[0])
		if !ok {
			return "", false
		}
		rightPath, ok := schemaFieldPath(node.children[1])
		if !ok {
			return "", false
		}
		if leftPath == "@" {
			return rightPath, true
		}
		if rightPath == "@" {
			return leftPath, true
		}
		return leftPath + "." + rightPath, true
	case ASTIndexExpression:
		if len(node.children) != 2 {
			return "", false
		}
		basePath, ok := schemaFieldPath(node.children[0])
		if !ok {
			return "", false
		}
		indexNode := node.children[1]
		switch indexNode.nodeType {
		case ASTIndex:
			index, ok := indexNode.value.(int)
			if !ok {
				return "", false
			}
			return fmt.Sprintf("%s[%d]", basePath, index), true
		case ASTSlice:
			return basePath + "[]", true
		default:
			return "", false
		}
	case ASTIdentity, ASTCurrentNode:
		return "@", true
	default:
		return "", false
	}
}

func (a *schemaAnalyzer) analyzeFunction(node ASTNode, input *staticType) (*staticType, error) {
	name, _ := node.value.(string)
	entry, exists := compileFunctionTable[name]
	if !exists {
		return nil, a.errorAt(node, staticErrUnknownFunction, fmt.Sprintf("unknown function %q", name))
	}
	if !isValidFunctionArity(entry.arguments, node.children) {
		return nil, a.errorAt(node, staticErrInvalidFuncArity, fmt.Sprintf("invalid arity for function %q", name))
	}
	switch name {
	case "map":
		return a.analyzeMapFunction(node, input, entry)
	case "max_by", "min_by", "sort_by":
		return a.analyzeByFunction(node, input, entry, name)
	case "not_null":
		return a.analyzeNotNullFunction(node, input)
	default:
		argTypes := make([]*staticType, len(node.children))
		for i, argNode := range node.children {
			argType, err := a.analyze(argNode, input)
			if err != nil {
				return nil, err
			}
			argTypes[i] = argType
			spec := functionArgSpecForIndex(entry.arguments, i)
			if err := a.validateFunctionArg(name, i, argNode, argType, spec); err != nil {
				return nil, err
			}
		}
		if name == "contains" {
			if err := a.validateContainsLiteralMembership(node, argTypes); err != nil {
				return nil, err
			}
		}
		return inferFunctionReturnType(name, argTypes), nil
	}
}

func (a *schemaAnalyzer) analyzeMapFunction(node ASTNode, input *staticType, entry functionEntry) (*staticType, error) {
	exprefArg := node.children[0]
	arrayArg := node.children[1]
	arrayType, err := a.analyze(arrayArg, input)
	if err != nil {
		return nil, err
	}
	if err := a.validateFunctionArg("map", 1, arrayArg, arrayType, entry.arguments[1]); err != nil {
		return nil, err
	}
	arrayElementType, err := a.ensureArrayElementType(arrayArg, arrayType)
	if err != nil {
		return nil, err
	}
	resultType, err := a.analyzeExpRefArg(exprefArg, arrayElementType)
	if err != nil {
		return nil, err
	}
	return staticArrayOf(resultType), nil
}

func (a *schemaAnalyzer) analyzeNotNullFunction(node ASTNode, input *staticType) (*staticType, error) {
	argTypes := make([]*staticType, 0, len(node.children))
	for _, argNode := range node.children {
		argType, err := a.analyze(argNode, input)
		if err != nil {
			return nil, err
		}
		argTypes = append(argTypes, argType)
		if !normalizeStaticType(argType).includes(staticMaskNull) {
			break
		}
	}
	return inferNotNullReturnType(argTypes), nil
}

func (a *schemaAnalyzer) analyzeByFunction(node ASTNode, input *staticType, entry functionEntry, name string) (*staticType, error) {
	arrayArg := node.children[0]
	exprefArg := node.children[1]
	arrayType, err := a.analyze(arrayArg, input)
	if err != nil {
		return nil, err
	}
	if err := a.validateFunctionArg(name, 0, arrayArg, arrayType, entry.arguments[0]); err != nil {
		return nil, err
	}
	arrayElementType, err := a.ensureArrayElementType(arrayArg, arrayType)
	if err != nil {
		return nil, err
	}
	exprefResultType, err := a.analyzeExpRefArg(exprefArg, arrayElementType)
	if err != nil {
		return nil, err
	}
	if err := a.validateByFunctionExprefResult(name, exprefArg, exprefResultType); err != nil {
		return nil, err
	}
	if name == "sort_by" {
		return normalizeStaticType(arrayType), nil
	}
	return arrayElementType, nil
}

func (a *schemaAnalyzer) analyzeExpRefArg(node ASTNode, elementType *staticType) (*staticType, error) {
	if node.nodeType != ASTExpRef || len(node.children) != 1 {
		return nil, a.errorAt(node, staticErrInvalidFuncArgType, "expected expression reference")
	}
	return a.analyze(node.children[0], elementType)
}

func (a *schemaAnalyzer) validateByFunctionExprefResult(functionName string, node ASTNode, resultType *staticType) error {
	normalized := normalizeStaticType(resultType)
	hasNullableRisk := normalized.includes(staticMaskNull)
	matchType := normalized
	if hasNullableRisk {
		matchType = staticWithoutNull(normalized)
		if staticTypeIsEmpty(matchType) {
			return a.errorAt(node, staticErrUnsafeOptionalArg, fmt.Sprintf("function %q argument 2 may evaluate to missing or null and can trigger invalid-type", functionName))
		}
	}
	match := evaluateArgMatch(matchType, []jpType{jpNumber, jpString})
	switch match {
	case argMatchYes:
		if hasNullableRisk {
			return a.errorAt(node, staticErrUnsafeOptionalArg, fmt.Sprintf("function %q argument 2 may evaluate to missing or null and can trigger invalid-type", functionName))
		}
		return nil
	case argMatchMaybe:
		return a.errorAt(node, staticErrUnverifiableType, fmt.Sprintf("cannot prove function %q expref result type", functionName))
	default:
		return a.errorAt(node, staticErrInvalidFuncArgType, fmt.Sprintf("function %q expects expref result to be number or string", functionName))
	}
}

func (a *schemaAnalyzer) ensureArrayElementType(node ASTNode, typ *staticType) (*staticType, error) {
	current := normalizeStaticType(typ)
	if !current.includes(staticMaskArray) {
		return nil, a.errorAt(node, staticErrInvalidFuncArgType, "expected array argument")
	}
	if !current.isDefinite(staticMaskArray) || current.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove function argument is array")
	}
	return current.array.itemType(), nil
}

func (a *schemaAnalyzer) validateFunctionArg(functionName string, argIndex int, node ASTNode, argType *staticType, spec argSpec) error {
	if len(spec.types) == 1 && spec.types[0] == jpExpref {
		if node.nodeType != ASTExpRef {
			return a.errorAt(node, staticErrInvalidFuncArgType, fmt.Sprintf("function %q argument %d must be expression reference", functionName, argIndex+1))
		}
		return nil
	}
	normalized := normalizeStaticType(argType)
	hasNullableRisk := normalized.includes(staticMaskNull) && !specAllowsNull(spec)
	matchType := normalized
	if hasNullableRisk {
		matchType = staticWithoutNull(normalized)
		if staticTypeIsEmpty(matchType) {
			return a.errorAt(node, staticErrUnsafeOptionalArg, fmt.Sprintf("function %q argument %d may be missing or null and can trigger invalid-type", functionName, argIndex+1))
		}
	}
	if typedArrayNullableRisk(matchType, spec.types) {
		return a.errorAt(node, staticErrUnsafeOptionalArg, fmt.Sprintf("function %q argument %d may contain null elements and can trigger invalid-type", functionName, argIndex+1))
	}
	match := evaluateArgMatch(matchType, spec.types)
	switch match {
	case argMatchYes:
		if hasNullableRisk {
			return a.errorAt(node, staticErrUnsafeOptionalArg, fmt.Sprintf("function %q argument %d may be missing or null and can trigger invalid-type", functionName, argIndex+1))
		}
		return nil
	case argMatchMaybe:
		return a.errorAt(node, staticErrUnverifiableType, fmt.Sprintf("cannot prove function %q argument %d type", functionName, argIndex+1))
	default:
		return a.errorAt(node, staticErrInvalidFuncArgType, fmt.Sprintf("invalid argument %d for function %q", argIndex+1, functionName))
	}
}

type argMatch int

const (
	argMatchNo argMatch = iota
	argMatchMaybe
	argMatchYes
)

func evaluateArgMatch(argType *staticType, expected []jpType) argMatch {
	best := argMatchNo
	for _, typ := range expected {
		match := matchJPType(argType, typ)
		if match > best {
			best = match
		}
		if best == argMatchYes {
			return best
		}
	}
	return best
}

func matchJPType(argType *staticType, expected jpType) argMatch {
	current := normalizeStaticType(argType)
	switch expected {
	case jpAny:
		return argMatchYes
	case jpString:
		return matchMask(current, staticMaskString)
	case jpNumber:
		return matchMask(current, staticMaskNumber)
	case jpArray:
		return matchMask(current, staticMaskArray)
	case jpObject:
		return matchMask(current, staticMaskObject)
	case jpArrayNumber:
		return matchTypedArray(current, staticMaskNumber)
	case jpArrayString:
		return matchTypedArray(current, staticMaskString)
	case jpExpref:
		return argMatchNo
	default:
		return argMatchNo
	}
}

func matchMask(typ *staticType, mask staticTypeMask) argMatch {
	if !typ.includes(mask) {
		return argMatchNo
	}
	if typ.isDefinite(mask) {
		return argMatchYes
	}
	return argMatchMaybe
}

func matchTypedArray(typ *staticType, itemMask staticTypeMask) argMatch {
	if !typ.includes(staticMaskArray) {
		return argMatchNo
	}
	if !typ.isDefinite(staticMaskArray) || typ.array == nil {
		return argMatchMaybe
	}
	items := typ.array.itemType()
	if !items.includes(itemMask) {
		return argMatchNo
	}
	if items.isDefinite(itemMask) {
		return argMatchYes
	}
	return argMatchMaybe
}

func typedArrayNullableRisk(argType *staticType, expected []jpType) bool {
	current := normalizeStaticType(argType)
	if !current.isDefinite(staticMaskArray) || current.array == nil {
		return false
	}
	items := current.array.itemType()
	if !items.includes(staticMaskNull) {
		return false
	}
	nonNullItems := staticWithoutNull(items)
	if staticTypeIsEmpty(nonNullItems) {
		return true
	}
	for _, typ := range expected {
		switch typ {
		case jpArrayNumber:
			if nonNullItems.isDefinite(staticMaskNumber) {
				return true
			}
		case jpArrayString:
			if nonNullItems.isDefinite(staticMaskString) {
				return true
			}
		}
	}
	return false
}

func isValidFunctionArity(arguments []argSpec, nodes []ASTNode) bool {
	if len(arguments) == 0 {
		return len(nodes) == 0
	}
	last := arguments[len(arguments)-1]
	if last.variadic {
		return len(nodes) >= len(arguments)
	}
	return len(nodes) == len(arguments)
}

func functionArgSpecForIndex(arguments []argSpec, index int) argSpec {
	if len(arguments) == 0 {
		return argSpec{types: []jpType{jpAny}}
	}
	if index < len(arguments) {
		return arguments[index]
	}
	last := arguments[len(arguments)-1]
	if last.variadic {
		return last
	}
	return last
}

func specAllowsNull(spec argSpec) bool {
	for _, typ := range spec.types {
		if typ == jpAny {
			return true
		}
	}
	return false
}

func inferFunctionReturnType(name string, args []*staticType) *staticType {
	switch name {
	case "length", "abs", "avg", "ceil", "floor", "sum":
		return staticNumberTypeValue
	case "starts_with", "contains", "ends_with":
		return staticBooleanTypeValue
	case "type", "to_string", "join":
		return staticStringTypeValue
	case "keys":
		return staticArrayOf(staticStringTypeValue)
	case "values":
		return staticArrayOf(staticAnyTypeValue)
	case "merge":
		return staticOpenObject()
	case "to_number":
		return inferToNumberReturnType(args)
	case "max", "min":
		return inferMinMaxReturnType(args)
	case "sort":
		if len(args) == 0 {
			return staticArrayOf(staticAnyTypeValue)
		}
		first := normalizeStaticType(args[0])
		if first.isDefinite(staticMaskArray) {
			return first
		}
		return staticArrayOf(staticAnyTypeValue)
	case "reverse":
		if len(args) == 0 {
			return staticAnyTypeValue
		}
		first := normalizeStaticType(args[0])
		if first.isDefinite(staticMaskString) || first.isDefinite(staticMaskArray) {
			return first
		}
		return staticAnyTypeValue
	case "to_array":
		if len(args) == 0 {
			return staticArrayOf(staticAnyTypeValue)
		}
		first := normalizeStaticType(args[0])
		if first.isDefinite(staticMaskArray) {
			return first
		}
		return staticArrayOf(staticAnyTypeValue)
	case "not_null":
		return inferNotNullReturnType(args)
	default:
		return staticAnyTypeValue
	}
}

func inferToNumberReturnType(args []*staticType) *staticType {
	if len(args) == 0 {
		return staticNullable(staticNumberTypeValue)
	}
	first := normalizeStaticType(args[0])
	if first.isDefinite(staticMaskNumber) {
		return staticNumberTypeValue
	}
	if first.includes(staticMaskNumber) || first.includes(staticMaskString) {
		return staticNullable(staticNumberTypeValue)
	}
	return staticNullTypeValue
}

func inferMinMaxReturnType(args []*staticType) *staticType {
	if len(args) == 0 {
		return staticAnyTypeValue
	}
	first := normalizeStaticType(args[0])
	if first.isDefinite(staticMaskArray) && first.array != nil {
		return staticNullable(first.array.itemType())
	}
	return staticAnyTypeValue
}

func inferNotNullReturnType(args []*staticType) *staticType {
	if len(args) == 0 {
		return staticAnyTypeValue
	}
	var result *staticType
	allMayBeNull := true
	for _, arg := range args {
		if !allMayBeNull {
			break
		}
		current := normalizeStaticType(arg)
		nonNull := staticWithoutNull(current)
		if !staticTypeIsEmpty(nonNull) {
			result = staticUnion(result, nonNull)
		}
		if !current.includes(staticMaskNull) {
			allMayBeNull = false
			break
		}
	}
	if allMayBeNull {
		result = staticUnion(result, staticNullTypeValue)
	}
	if result == nil {
		return staticNullTypeValue
	}
	return normalizeStaticType(result)
}

func staticTypeFromLiteral(value interface{}) *staticType {
	if literal, ok := scalarLiteralFromInterface(value); ok {
		return staticTypeFromScalarLiteral(literal)
	}
	switch value.(type) {
	case []interface{}:
		return staticArrayOf(staticAnyTypeValue)
	case map[string]interface{}:
		return staticOpenObject()
	default:
		return staticAnyTypeValue
	}
}

func staticTypeFromScalarLiteral(literal scalarLiteral) *staticType {
	result := &staticType{
		mask:       schemaKindMask(literal.toSchemaKind()),
		constValue: &literal,
	}
	if literal.kind == scalarLiteralString && isValidDateString(literal.stringValue) {
		// Preserve date-looking literals so unions such as
		// not_null(optionalDate, "2026-03-01") retain the date format signal.
		result.stringFormat = stringFormatDate
		result.formatSource = stringFormatSourceLiteral
	}
	return result
}

func dateLiteralValue(node ASTNode) (string, bool) {
	if node.nodeType != ASTLiteral {
		return "", false
	}
	value, ok := node.value.(string)
	if !ok || !isValidDateString(value) {
		return "", false
	}
	return value, true
}

func staticArrayOf(items *staticType) *staticType {
	return &staticType{
		mask:  staticMaskArray,
		array: &staticArrayType{items: normalizeStaticType(items)},
	}
}

func staticNullable(typ *staticType) *staticType {
	current := normalizeStaticType(typ)
	if current.includes(staticMaskNull) {
		return current
	}
	return staticUnion(current, staticNullTypeValue)
}

func staticWithoutNull(typ *staticType) *staticType {
	current := normalizeStaticType(typ)
	if !current.includes(staticMaskNull) {
		return current
	}
	mask := current.mask &^ staticMaskNull
	result := &staticType{
		mask:         mask,
		object:       current.object,
		array:        current.array,
		constValue:   current.constValue,
		enumValues:   current.enumValues,
		stringFormat: current.stringFormat,
		formatSource: current.formatSource,
	}
	if mask&staticMaskObject == 0 {
		result.object = nil
	}
	if mask&staticMaskArray == 0 {
		result.array = nil
	}
	// Constraints and string formats are only sound for exact string/scalar types.
	if mask == 0 || mask&(mask-1) != 0 || mask&(staticMaskObject|staticMaskArray) != 0 {
		result.constValue = nil
		result.enumValues = nil
		result.stringFormat = stringFormatNone
		result.formatSource = stringFormatSourceNone
	}
	if mask != staticMaskString {
		result.stringFormat = stringFormatNone
		result.formatSource = stringFormatSourceNone
	}
	return result
}

func staticTypeIsEmpty(typ *staticType) bool {
	current := normalizeStaticType(typ)
	return current.mask == 0
}

func staticOpenObject() *staticType {
	return &staticType{
		mask: staticMaskObject,
		object: &staticObjectType{
			additionalMode: additionalPropertiesAllowOpen,
		},
	}
}

func staticClosedObject(properties map[string]*staticType) *staticType {
	return &staticType{
		mask: staticMaskObject,
		object: &staticObjectType{
			properties:     properties,
			additionalMode: additionalPropertiesForbid,
		},
	}
}

func staticUnion(left, right *staticType) *staticType {
	if left == nil {
		return normalizeStaticType(right)
	}
	if right == nil {
		return normalizeStaticType(left)
	}
	if left == right {
		return left
	}
	l := normalizeStaticType(left)
	r := normalizeStaticType(right)
	union := &staticType{mask: l.mask | r.mask}
	if union.mask&staticMaskArray != 0 {
		switch {
		case l.includes(staticMaskArray) && r.includes(staticMaskArray):
			if l.array != nil && r.array != nil {
				union.array = &staticArrayType{items: staticUnion(l.array.itemType(), r.array.itemType())}
			}
		case l.includes(staticMaskArray):
			union.array = l.array
		case r.includes(staticMaskArray):
			union.array = r.array
		}
	}
	if union.mask&staticMaskObject != 0 {
		switch {
		case l.includes(staticMaskObject) && r.includes(staticMaskObject):
			if l.object != nil && r.object != nil && l.object == r.object {
				union.object = l.object
			}
		case l.includes(staticMaskObject):
			union.object = l.object
		case r.includes(staticMaskObject):
			union.object = r.object
		}
	}
	union.constValue, union.enumValues = unionScalarConstraints(l, r, union.mask)
	union.stringFormat, union.formatSource = unionStringFormat(l, r, union.mask)
	return union
}

func unionScalarConstraints(left, right *staticType, unionMask staticTypeMask) (*scalarLiteral, *scalarLiteralSet) {
	if unionMask&(staticMaskObject|staticMaskArray) != 0 {
		return nil, nil
	}
	if left.mask == staticMaskNull {
		return right.constValue, right.enumValues
	}
	if right.mask == staticMaskNull {
		return left.constValue, left.enumValues
	}
	if left.constValue != nil && right.constValue != nil && left.constValue.equals(*right.constValue) {
		return left.constValue, nil
	}
	if left.enumValues != nil && right.enumValues != nil && scalarLiteralSetsEqual(left.enumValues, right.enumValues) {
		return nil, left.enumValues
	}
	return nil, nil
}

func scalarLiteralSetsEqual(left, right *scalarLiteralSet) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(left.values) != len(right.values) {
		return false
	}
	for _, value := range left.values {
		if !right.contains(value) {
			return false
		}
	}
	return true
}

func unionStringFormat(left, right *staticType, unionMask staticTypeMask) (stringFormat, stringFormatSource) {
	if unionMask&^(staticMaskString|staticMaskNull) != 0 || unionMask&staticMaskString == 0 {
		return stringFormatNone, stringFormatSourceNone
	}
	leftFormat, leftSource, leftHasString := left.nonNullStringFormat()
	rightFormat, rightSource, rightHasString := right.nonNullStringFormat()
	switch {
	case leftHasString && rightHasString:
		if leftFormat == rightFormat {
			source := leftSource | rightSource
			// Literal-only unions must not manufacture schema-level date semantics.
			if source == stringFormatSourceLiteral {
				return stringFormatNone, stringFormatSourceNone
			}
			return leftFormat, source
		}
		return stringFormatNone, stringFormatSourceNone
	case leftHasString:
		if leftSource == stringFormatSourceLiteral {
			return stringFormatNone, stringFormatSourceNone
		}
		return leftFormat, leftSource
	case rightHasString:
		if rightSource == stringFormatSourceLiteral {
			return stringFormatNone, stringFormatSourceNone
		}
		return rightFormat, rightSource
	default:
		return stringFormatNone, stringFormatSourceNone
	}
}

func normalizeStaticType(typ *staticType) *staticType {
	if typ == nil {
		return staticAnyTypeValue
	}
	return typ
}

func (t *staticType) nonNullStringFormat() (stringFormat, stringFormatSource, bool) {
	current := staticWithoutNull(normalizeStaticType(t))
	if current.mask != staticMaskString {
		return stringFormatNone, stringFormatSourceNone, false
	}
	return current.stringFormat, current.formatSource, true
}

func (t *staticType) includes(mask staticTypeMask) bool {
	current := normalizeStaticType(t)
	return current.mask&mask != 0
}

func (t *staticType) isDefinite(mask staticTypeMask) bool {
	current := normalizeStaticType(t)
	return current.mask == mask
}

func (t *staticType) hasStringFormat(format stringFormat) bool {
	current := normalizeStaticType(t)
	return current.isDefinite(staticMaskString) &&
		current.stringFormat == format &&
		current.formatSource != stringFormatSourceLiteral
}

func (t *staticType) truthiness() staticTruthiness {
	current := normalizeStaticType(t)
	canBeTruthy := current.canBeTruthy()
	canBeFalsey := current.canBeFalsey()
	switch {
	case canBeTruthy && canBeFalsey:
		return staticTruthinessUnknown
	case canBeTruthy:
		return staticTruthinessAlwaysTrue
	case canBeFalsey:
		return staticTruthinessAlwaysFalse
	default:
		return staticTruthinessUnknown
	}
}

func (t *staticType) canBeTruthy() bool {
	current := normalizeStaticType(t)
	if current.mask&staticMaskNumber != 0 {
		return true
	}
	if current.mask&staticMaskArray != 0 {
		return true
	}
	if current.mask&staticMaskObject != 0 {
		return true
	}
	if current.mask&staticMaskBoolean != 0 {
		if current.constValue != nil && current.constValue.kind == scalarLiteralBoolean {
			return current.constValue.boolValue
		}
		if current.enumValues != nil {
			return current.enumValues.contains(scalarLiteral{kind: scalarLiteralBoolean, boolValue: true})
		}
		return true
	}
	if current.mask&staticMaskString != 0 {
		if current.constValue != nil && current.constValue.kind == scalarLiteralString {
			return current.constValue.stringValue != ""
		}
		if current.enumValues != nil {
			for _, value := range current.enumValues.values {
				if value.kind == scalarLiteralString && value.stringValue != "" {
					return true
				}
			}
			return false
		}
		return true
	}
	return false
}

func (t *staticType) canBeFalsey() bool {
	current := normalizeStaticType(t)
	if current.mask&staticMaskNull != 0 {
		return true
	}
	if current.mask&staticMaskArray != 0 {
		return true
	}
	if current.mask&staticMaskObject != 0 {
		if current.object == nil || len(current.object.required) == 0 {
			return true
		}
	}
	if current.mask&staticMaskBoolean != 0 {
		if current.constValue != nil && current.constValue.kind == scalarLiteralBoolean {
			return !current.constValue.boolValue
		}
		if current.enumValues != nil {
			return current.enumValues.contains(scalarLiteral{kind: scalarLiteralBoolean, boolValue: false})
		}
		return true
	}
	if current.mask&staticMaskString != 0 {
		if current.constValue != nil && current.constValue.kind == scalarLiteralString {
			return current.constValue.stringValue == ""
		}
		if current.enumValues != nil {
			return current.enumValues.contains(scalarLiteral{kind: scalarLiteralString, stringValue: ""})
		}
		return true
	}
	return false
}

func (t *staticArrayType) itemType() *staticType {
	if t == nil || t.items == nil {
		return staticAnyTypeValue
	}
	return normalizeStaticType(t.items)
}

func (t *staticObjectType) valuesType() *staticType {
	if t == nil {
		return staticAnyTypeValue
	}
	if t.additionalMode != additionalPropertiesForbid {
		return staticAnyTypeValue
	}
	var merged *staticType
	for _, value := range t.properties {
		merged = staticUnion(merged, value)
	}
	if merged == nil {
		return staticAnyTypeValue
	}
	return merged
}

func (t *staticObjectType) isRequired(name string) bool {
	if t == nil || len(t.required) == 0 {
		return false
	}
	_, exists := t.required[name]
	return exists
}

func (a *schemaAnalyzer) analyzeMultiSelectList(node ASTNode, input *staticType) (*staticType, error) {
	var merged *staticType
	for _, child := range node.children {
		childType, err := a.analyze(child, input)
		if err != nil {
			return nil, err
		}
		merged = staticUnion(merged, childType)
	}
	if merged == nil {
		merged = staticAnyTypeValue
	}
	return staticArrayOf(merged), nil
}

func (a *schemaAnalyzer) analyzeMultiSelectHash(node ASTNode, input *staticType) (*staticType, error) {
	properties := make(map[string]*staticType, len(node.children))
	for _, child := range node.children {
		childType, err := a.analyze(child, input)
		if err != nil {
			return nil, err
		}
		key, _ := child.value.(string)
		properties[key] = childType
	}
	return staticClosedObject(properties), nil
}

func (a *schemaAnalyzer) errorAt(node ASTNode, code, message string) *StaticError {
	return newStaticError(code, a.expression, node.offset, message)
}
