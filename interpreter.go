package jmespath

import (
	"errors"
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"
)

/* This is a tree based interpreter.  It walks the AST and directly
   interprets the AST to search through a JSON document.
*/

type treeInterpreter struct {
	fCall           *functionCaller
	comparatorPlans map[int]comparatorPlan
}

func newInterpreter() *treeInterpreter {
	return newInterpreterWithComparatorPlans(nil)
}

func newInterpreterWithComparatorPlans(comparatorPlans map[int]comparatorPlan) *treeInterpreter {
	interpreter := treeInterpreter{}
	interpreter.fCall = newFunctionCaller()
	interpreter.comparatorPlans = comparatorPlans
	return &interpreter
}

type expRef struct {
	ref ASTNode
}

// Execute takes an ASTNode and input data and interprets the AST directly.
// It will produce the result of applying the JMESPath expression associated
// with the ASTNode to the input data "value".
func (intr *treeInterpreter) Execute(node ASTNode, value interface{}) (interface{}, error) {
	switch node.nodeType {
	case ASTComparator:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		right, err := intr.Execute(node.children[1], value)
		if err != nil {
			return nil, err
		}
		switch node.value {
		case tEQ:
			return objsEqual(left, right), nil
		case tNE:
			return !objsEqual(left, right), nil
		}
		plan := intr.comparatorPlan(node)
		switch plan.kind {
		case orderedValueKindDate:
			return intr.compareDates(node.value.(tokType), left, right, plan)
		case orderedValueKindNumber:
			return compareNumbers(node.value.(tokType), left, right)
		}
		return compareNumbers(node.value.(tokType), left, right)
	case ASTExpRef:
		return expRef{ref: node.children[0]}, nil
	case ASTFunctionExpression:
		resolvedArgs := []interface{}{}
		for _, arg := range node.children {
			current, err := intr.Execute(arg, value)
			if err != nil {
				return nil, err
			}
			resolvedArgs = append(resolvedArgs, current)
		}
		return intr.fCall.CallFunction(node.value.(string), resolvedArgs, intr)
	case ASTField:
		if m, ok := value.(map[string]interface{}); ok {
			key := node.value.(string)
			return m[key], nil
		}
		return intr.fieldFromStruct(node.value.(string), value)
	case ASTFilterProjection:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, nil
		}
		sliceType, ok := left.([]interface{})
		if !ok {
			if isSliceType(left) {
				return intr.filterProjectionWithReflection(node, left)
			}
			return nil, nil
		}
		compareNode := node.children[2]
		collected := []interface{}{}
		for _, element := range sliceType {
			result, err := intr.Execute(compareNode, element)
			if err != nil {
				return nil, err
			}
			if !isFalse(result) {
				current, err := intr.Execute(node.children[1], element)
				if err != nil {
					return nil, err
				}
				if current != nil {
					collected = append(collected, current)
				}
			}
		}
		return collected, nil
	case ASTFlatten:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, nil
		}
		sliceType, ok := left.([]interface{})
		if !ok {
			// If we can't type convert to []interface{}, there's
			// a chance this could still work via reflection if we're
			// dealing with user provided types.
			if isSliceType(left) {
				return intr.flattenWithReflection(left)
			}
			return nil, nil
		}
		flattened := []interface{}{}
		for _, element := range sliceType {
			if elementSlice, ok := element.([]interface{}); ok {
				flattened = append(flattened, elementSlice...)
			} else if isSliceType(element) {
				reflectFlat := []interface{}{}
				v := reflect.ValueOf(element)
				for i := 0; i < v.Len(); i++ {
					reflectFlat = append(reflectFlat, v.Index(i).Interface())
				}
				flattened = append(flattened, reflectFlat...)
			} else {
				flattened = append(flattened, element)
			}
		}
		return flattened, nil
	case ASTIdentity, ASTCurrentNode:
		return value, nil
	case ASTIndex:
		if sliceType, ok := value.([]interface{}); ok {
			index := node.value.(int)
			if index < 0 {
				index += len(sliceType)
			}
			if index < len(sliceType) && index >= 0 {
				return sliceType[index], nil
			}
			return nil, nil
		}
		// Otherwise try via reflection.
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice {
			index := node.value.(int)
			if index < 0 {
				index += rv.Len()
			}
			if index < rv.Len() && index >= 0 {
				v := rv.Index(index)
				return v.Interface(), nil
			}
		}
		return nil, nil
	case ASTKeyValPair:
		return intr.Execute(node.children[0], value)
	case ASTLiteral:
		return node.value, nil
	case ASTMultiSelectHash:
		if value == nil {
			return nil, nil
		}
		collected := make(map[string]interface{})
		for _, child := range node.children {
			current, err := intr.Execute(child, value)
			if err != nil {
				return nil, err
			}
			key := child.value.(string)
			collected[key] = current
		}
		return collected, nil
	case ASTMultiSelectList:
		if value == nil {
			return nil, nil
		}
		collected := []interface{}{}
		for _, child := range node.children {
			current, err := intr.Execute(child, value)
			if err != nil {
				return nil, err
			}
			collected = append(collected, current)
		}
		return collected, nil
	case ASTOrExpression:
		matched, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		if isFalse(matched) {
			matched, err = intr.Execute(node.children[1], value)
			if err != nil {
				return nil, err
			}
		}
		return matched, nil
	case ASTAndExpression:
		matched, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		if isFalse(matched) {
			return matched, nil
		}
		return intr.Execute(node.children[1], value)
	case ASTNotExpression:
		matched, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		if isFalse(matched) {
			return true, nil
		}
		return false, nil
	case ASTPipe:
		result := value
		var err error
		for _, child := range node.children {
			result, err = intr.Execute(child, result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	case ASTProjection:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		sliceType, ok := left.([]interface{})
		if !ok {
			if isSliceType(left) {
				return intr.projectWithReflection(node, left)
			}
			return nil, nil
		}
		collected := []interface{}{}
		var current interface{}
		for _, element := range sliceType {
			current, err = intr.Execute(node.children[1], element)
			if err != nil {
				return nil, err
			}
			if current != nil {
				collected = append(collected, current)
			}
		}
		return collected, nil
	case ASTSubexpression, ASTIndexExpression:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, err
		}
		return intr.Execute(node.children[1], left)
	case ASTSlice:
		sliceType, ok := value.([]interface{})
		if !ok {
			if isSliceType(value) {
				return intr.sliceWithReflection(node, value)
			}
			return nil, nil
		}
		parts := node.value.([]*int)
		sliceParams := make([]sliceParam, 3)
		for i, part := range parts {
			if part != nil {
				sliceParams[i].Specified = true
				sliceParams[i].N = *part
			}
		}
		return slice(sliceType, sliceParams)
	case ASTValueProjection:
		left, err := intr.Execute(node.children[0], value)
		if err != nil {
			return nil, nil
		}
		mapType, ok := left.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		values := make([]interface{}, len(mapType))
		for _, value := range mapType {
			values = append(values, value)
		}
		collected := []interface{}{}
		for _, element := range values {
			current, err := intr.Execute(node.children[1], element)
			if err != nil {
				return nil, err
			}
			if current != nil {
				collected = append(collected, current)
			}
		}
		return collected, nil
	}
	return nil, errors.New("Unknown AST node: " + node.nodeType.String())
}

func (intr *treeInterpreter) comparatorPlan(node ASTNode) comparatorPlan {
	if intr == nil || len(intr.comparatorPlans) == 0 {
		return comparatorPlan{kind: orderedValueKindNumber}
	}
	if plan, exists := intr.comparatorPlans[node.offset]; exists {
		return plan
	}
	return comparatorPlan{kind: orderedValueKindNumber}
}

func compareNumbers(operator tokType, left, right interface{}) (interface{}, error) {
	leftNum, ok := left.(float64)
	if !ok {
		return nil, nil
	}
	rightNum, ok := right.(float64)
	if !ok {
		return nil, nil
	}
	switch operator {
	case tGT:
		return leftNum > rightNum, nil
	case tGTE:
		return leftNum >= rightNum, nil
	case tLT:
		return leftNum < rightNum, nil
	case tLTE:
		return leftNum <= rightNum, nil
	default:
		return nil, nil
	}
}

func (intr *treeInterpreter) compareDates(operator tokType, left, right interface{}, plan comparatorPlan) (interface{}, error) {
	leftDate, err := runtimeDateValue(left, plan.leftDateLiteral)
	if err != nil {
		return nil, err
	}
	rightDate, err := runtimeDateValue(right, plan.rightDateLiteral)
	if err != nil {
		return nil, err
	}
	switch operator {
	// Valid YYYY-MM-DD strings are lexicographically ordered the same way as
	// their calendar dates, so we can compare strings directly and skip time.Parse.
	case tGT:
		return leftDate > rightDate, nil
	case tGTE:
		return leftDate >= rightDate, nil
	case tLT:
		return leftDate < rightDate, nil
	case tLTE:
		return leftDate <= rightDate, nil
	default:
		return nil, nil
	}
}

func runtimeDateValue(value interface{}, prevalidatedLiteral string) (string, error) {
	if prevalidatedLiteral != "" {
		return prevalidatedLiteral, nil
	}
	dateString, ok := value.(string)
	if !ok {
		return "", invalidTypeError("expected date string in YYYY-MM-DD format")
	}
	// Validate the runtime value without allocating/parsing time.Time on the hot path.
	if err := validateDateString(dateString); err != nil {
		return "", invalidTypeError(fmt.Sprintf("expected date string in YYYY-MM-DD format: %v", err))
	}
	return dateString, nil
}

func invalidTypeError(message string) error {
	return fmt.Errorf("invalid-type: %s", message)
}

func (intr *treeInterpreter) fieldFromStruct(key string, value interface{}) (interface{}, error) {
	rv := reflect.ValueOf(value)
	first, n := utf8.DecodeRuneInString(key)
	fieldName := string(unicode.ToUpper(first)) + key[n:]
	if rv.Kind() == reflect.Struct {
		v := rv.FieldByName(fieldName)
		if !v.IsValid() {
			return nil, nil
		}
		return v.Interface(), nil
	} else if rv.Kind() == reflect.Ptr {
		// Handle multiple levels of indirection?
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
		v := rv.FieldByName(fieldName)
		if !v.IsValid() {
			return nil, nil
		}
		return v.Interface(), nil
	}
	return nil, nil
}

func (intr *treeInterpreter) flattenWithReflection(value interface{}) (interface{}, error) {
	v := reflect.ValueOf(value)
	flattened := []interface{}{}
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i).Interface()
		if reflect.TypeOf(element).Kind() == reflect.Slice {
			// Then insert the contents of the element
			// slice into the flattened slice,
			// i.e flattened = append(flattened, mySlice...)
			elementV := reflect.ValueOf(element)
			for j := 0; j < elementV.Len(); j++ {
				flattened = append(
					flattened, elementV.Index(j).Interface())
			}
		} else {
			flattened = append(flattened, element)
		}
	}
	return flattened, nil
}

func (intr *treeInterpreter) sliceWithReflection(node ASTNode, value interface{}) (interface{}, error) {
	v := reflect.ValueOf(value)
	parts := node.value.([]*int)
	sliceParams := make([]sliceParam, 3)
	for i, part := range parts {
		if part != nil {
			sliceParams[i].Specified = true
			sliceParams[i].N = *part
		}
	}
	final := []interface{}{}
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i).Interface()
		final = append(final, element)
	}
	return slice(final, sliceParams)
}

func (intr *treeInterpreter) filterProjectionWithReflection(node ASTNode, value interface{}) (interface{}, error) {
	compareNode := node.children[2]
	collected := []interface{}{}
	v := reflect.ValueOf(value)
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i).Interface()
		result, err := intr.Execute(compareNode, element)
		if err != nil {
			return nil, err
		}
		if !isFalse(result) {
			current, err := intr.Execute(node.children[1], element)
			if err != nil {
				return nil, err
			}
			if current != nil {
				collected = append(collected, current)
			}
		}
	}
	return collected, nil
}

func (intr *treeInterpreter) projectWithReflection(node ASTNode, value interface{}) (interface{}, error) {
	collected := []interface{}{}
	v := reflect.ValueOf(value)
	for i := 0; i < v.Len(); i++ {
		element := v.Index(i).Interface()
		result, err := intr.Execute(node.children[1], element)
		if err != nil {
			return nil, err
		}
		if result != nil {
			collected = append(collected, result)
		}
	}
	return collected, nil
}
