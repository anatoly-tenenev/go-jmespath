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
	mask       staticTypeMask
	object     *staticObjectType
	array      *staticArrayType
	constValue *scalarLiteral
	enumValues *scalarLiteralSet
}

type staticObjectType struct {
	properties       map[string]*staticType
	required         map[string]struct{}
	additionalMode   additionalPropertiesMode
	additionalSchema *staticType
}

type staticArrayType struct {
	items *staticType
}

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
	expression string
}

func analyzeExpressionAgainstSchema(expression string, ast ASTNode, cs *CompiledSchema) (*staticType, error) {
	if cs == nil || cs.root == nil {
		return nil, unsupportedSchemaError("$", "compiled schema is nil")
	}
	rootType := cs.staticRoot
	if rootType == nil {
		rootType = staticFromSchema(cs.root)
	}
	analyzer := &schemaAnalyzer{expression: expression}
	result, err := analyzer.analyze(ast, rootType)
	if err != nil {
		return nil, err
	}
	return normalizeStaticType(result), nil
}

func (a *schemaAnalyzer) analyze(node ASTNode, input *staticType) (*staticType, error) {
	switch node.nodeType {
	case ASTEmpty, ASTCurrentNode, ASTIdentity:
		return normalizeStaticType(input), nil
	case ASTField:
		return a.analyzeField(node, input)
	case ASTSubexpression, ASTPipe:
		return a.analyzeSequential(node, input)
	case ASTIndexExpression:
		return a.analyzeIndexExpression(node, input)
	case ASTProjection:
		return a.analyzeProjection(node, input)
	case ASTFilterProjection:
		return a.analyzeFilterProjection(node, input)
	case ASTFlatten:
		return a.analyzeFlatten(node, input)
	case ASTValueProjection:
		return a.analyzeValueProjection(node, input)
	case ASTComparator:
		return a.analyzeComparator(node, input)
	case ASTFunctionExpression:
		return a.analyzeFunction(node, input)
	case ASTExpRef:
		if len(node.children) == 1 {
			_, err := a.analyze(node.children[0], input)
			if err != nil {
				return nil, err
			}
		}
		return staticAnyTypeValue, nil
	case ASTLiteral:
		return staticTypeFromLiteral(node.value), nil
	case ASTMultiSelectList:
		return a.analyzeMultiSelectList(node, input)
	case ASTMultiSelectHash:
		return a.analyzeMultiSelectHash(node, input)
	case ASTKeyValPair:
		if len(node.children) == 0 {
			return staticAnyTypeValue, nil
		}
		return a.analyze(node.children[0], input)
	case ASTOrExpression, ASTAndExpression:
		return a.analyzeLogical(node, input)
	case ASTNotExpression:
		if len(node.children) == 0 {
			return staticBooleanTypeValue, nil
		}
		_, err := a.analyze(node.children[0], input)
		if err != nil {
			return nil, err
		}
		return staticBooleanTypeValue, nil
	case ASTSlice, ASTIndex:
		return normalizeStaticType(input), nil
	default:
		return staticAnyTypeValue, nil
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
	right, err := a.analyze(node.children[1], input)
	if err != nil {
		return nil, err
	}
	return staticUnion(left, right), nil
}

func (a *schemaAnalyzer) analyzeField(node ASTNode, input *staticType) (*staticType, error) {
	target := normalizeStaticType(input)
	if !target.includes(staticMaskObject) {
		return nil, a.errorAt(node, staticErrInvalidFieldTarget, "field access requires object target")
	}
	if !target.isDefinite(staticMaskObject) || target.object == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove field target is object")
	}
	name, _ := node.value.(string)
	if target.object.properties != nil {
		if value, exists := target.object.properties[name]; exists {
			return value, nil
		}
	}
	switch target.object.additionalMode {
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
	if !target.isDefinite(staticMaskArray) || target.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove index target is array")
	}
	itemType := target.array.itemType()
	if node.children[1].nodeType == ASTSlice {
		return staticArrayOf(itemType), nil
	}
	return itemType, nil
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
	if !target.isDefinite(staticMaskArray) || target.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove projection target is array")
	}
	result, err := a.analyze(node.children[1], target.array.itemType())
	if err != nil {
		return nil, err
	}
	return staticArrayOf(result), nil
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
	if !target.isDefinite(staticMaskArray) || target.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove filter projection target is array")
	}
	elementType := target.array.itemType()
	_, err = a.analyze(node.children[2], elementType)
	if err != nil {
		return nil, err
	}
	result, err := a.analyze(node.children[1], elementType)
	if err != nil {
		return nil, err
	}
	return staticArrayOf(result), nil
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
	if !target.isDefinite(staticMaskArray) || target.array == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove flatten target is array")
	}
	itemType := target.array.itemType()
	if itemType.isDefinite(staticMaskArray) && itemType.array != nil {
		return staticArrayOf(itemType.array.itemType()), nil
	}
	if !itemType.includes(staticMaskArray) {
		return staticArrayOf(itemType), nil
	}
	merged := staticAnyTypeValue
	if itemType.array != nil {
		merged = itemType.array.itemType()
	}
	nonArrayMask := itemType.mask &^ staticMaskArray
	if nonArrayMask != 0 {
		merged = staticUnion(merged, &staticType{mask: nonArrayMask})
	}
	return staticArrayOf(merged), nil
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
	if !target.isDefinite(staticMaskObject) || target.object == nil {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove value projection target is object")
	}
	valuesType := target.object.valuesType()
	result, err := a.analyze(node.children[1], valuesType)
	if err != nil {
		return nil, err
	}
	return staticArrayOf(result), nil
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
	if !left.includes(staticMaskNumber) || !right.includes(staticMaskNumber) {
		return nil, a.errorAt(node, staticErrInvalidComparator, "comparator requires number operands")
	}
	if !left.isDefinite(staticMaskNumber) || !right.isDefinite(staticMaskNumber) {
		return nil, a.errorAt(node, staticErrUnverifiableType, "cannot prove comparator operands are numbers")
	}
	return staticBooleanTypeValue, nil
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
	match := evaluateArgMatch(exprefResultType, []jpType{jpNumber, jpString})
	if match == argMatchNo {
		return nil, a.errorAt(exprefArg, staticErrInvalidFuncArgType, fmt.Sprintf("function %q expects expref result to be number or string", name))
	}
	if match == argMatchMaybe {
		return nil, a.errorAt(exprefArg, staticErrUnverifiableType, fmt.Sprintf("cannot prove function %q expref result type", name))
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
	match := evaluateArgMatch(argType, spec.types)
	switch match {
	case argMatchYes:
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

func inferFunctionReturnType(name string, args []*staticType) *staticType {
	switch name {
	case "length", "abs", "avg", "ceil", "floor", "sum", "to_number":
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
	case "max", "min":
		if len(args) == 0 {
			return staticAnyTypeValue
		}
		first := normalizeStaticType(args[0])
		if first.isDefinite(staticMaskArray) && first.array != nil {
			return first.array.itemType()
		}
		return staticAnyTypeValue
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
		var merged *staticType
		for _, arg := range args {
			merged = staticUnion(merged, arg)
		}
		if merged == nil {
			return staticAnyTypeValue
		}
		return merged
	default:
		return staticAnyTypeValue
	}
}

func staticTypeFromLiteral(value interface{}) *staticType {
	switch value.(type) {
	case nil:
		return staticNullTypeValue
	case string:
		return staticStringTypeValue
	case float64:
		return staticNumberTypeValue
	case bool:
		return staticBooleanTypeValue
	case []interface{}:
		return staticArrayOf(staticAnyTypeValue)
	case map[string]interface{}:
		return staticOpenObject()
	default:
		return staticAnyTypeValue
	}
}

func staticArrayOf(items *staticType) *staticType {
	return &staticType{
		mask:  staticMaskArray,
		array: &staticArrayType{items: normalizeStaticType(items)},
	}
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
	if union.mask == staticMaskArray && l.array != nil && r.array != nil {
		union.array = &staticArrayType{items: staticUnion(l.array.itemType(), r.array.itemType())}
	}
	if union.mask == staticMaskObject && l.object != nil && r.object != nil && l.object == r.object {
		union.object = l.object
	}
	return union
}

func normalizeStaticType(typ *staticType) *staticType {
	if typ == nil {
		return staticAnyTypeValue
	}
	return typ
}

func (t *staticType) includes(mask staticTypeMask) bool {
	current := normalizeStaticType(t)
	return current.mask&mask != 0
}

func (t *staticType) isDefinite(mask staticTypeMask) bool {
	current := normalizeStaticType(t)
	return current.mask == mask
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
