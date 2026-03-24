package jmespath

import (
	"fmt"
	"reflect"
)

var unsupportedSchemaKeywords = map[string]struct{}{
	"$ref":                  {},
	"oneOf":                 {},
	"anyOf":                 {},
	"allOf":                 {},
	"if":                    {},
	"then":                  {},
	"else":                  {},
	"prefixItems":           {},
	"patternProperties":     {},
	"unevaluatedProperties": {},
}

// CompileSchema compiles a supported subset of JSON schema into an internal IR.
func CompileSchema(schema JSONSchema) (*CompiledSchema, error) {
	if schema == nil {
		return nil, unsupportedSchemaError("$", "schema must be an object")
	}
	root, err := compileSchemaNode(map[string]interface{}(schema), "$")
	if err != nil {
		return nil, err
	}
	cs := &CompiledSchema{root: root}
	cs.staticRoot = staticFromSchema(root)
	return cs, nil
}

func compileSchemaNode(raw map[string]interface{}, path string) (*schemaNode, error) {
	if raw == nil {
		return nil, unsupportedSchemaError(path, "schema must be an object")
	}
	for key := range raw {
		if _, exists := unsupportedSchemaKeywords[key]; exists {
			return nil, unsupportedSchemaError(path, fmt.Sprintf("keyword %q is not supported", key))
		}
	}

	kind, hasType, err := parseSchemaKind(raw["type"], path)
	if err != nil {
		return nil, err
	}

	constValue, hasConst, err := compileConstValue(raw, path)
	if err != nil {
		return nil, err
	}

	enumValues, hasEnum, err := compileEnumValues(raw, path)
	if err != nil {
		return nil, err
	}

	hasConstraint := hasConst || hasEnum
	var constraintKind schemaKind
	if hasConst {
		constraintKind = constValue.toSchemaKind()
	}
	if hasEnum {
		enumKind, ok := enumValues.kind()
		if !ok {
			return nil, unsupportedSchemaError(path, "enum must have at least one value")
		}
		if hasConst && constValue.kind != enumKind {
			return nil, unsupportedSchemaError(path, "const and enum must have matching scalar types")
		}
		if hasConst && !enumValues.contains(*constValue) {
			return nil, unsupportedSchemaError(path, "const value must be present in enum")
		}
		constraintKind = scalarLiteral{kind: enumKind}.toSchemaKind()
	}

	rawProperties, hasProperties := raw["properties"]
	properties, err := compileProperties(rawProperties, path)
	if err != nil {
		return nil, err
	}
	if !hasProperties {
		properties = nil
	}

	required, hasRequired, err := compileRequired(raw["required"], path)
	if err != nil {
		return nil, err
	}
	if !hasRequired {
		required = nil
	}

	items, hasItems, err := compileItems(raw["items"], path)
	if err != nil {
		return nil, err
	}

	additionalMode, additionalSchema, hasAdditional, err := compileAdditionalProperties(raw["additionalProperties"], path)
	if err != nil {
		return nil, err
	}
	if !hasAdditional {
		additionalMode = additionalPropertiesAllowOpen
		additionalSchema = nil
	}

	if !hasType {
		switch {
		case hasProperties || hasRequired || hasAdditional:
			kind = schemaKindObject
		case hasItems:
			kind = schemaKindArray
		case hasConstraint:
			kind = constraintKind
		default:
			return nil, unsupportedSchemaError(path, "missing supported type")
		}
	}

	if hasConstraint {
		switch kind {
		case schemaKindObject, schemaKindArray:
			return nil, unsupportedSchemaError(path, "const/enum for object or array is not supported")
		case schemaKindString, schemaKindNumber, schemaKindBoolean, schemaKindNull:
			if kind != constraintKind {
				return nil, unsupportedSchemaError(path, "schema type does not match const/enum values")
			}
		default:
			return nil, unsupportedSchemaError(path, "unsupported scalar type for const/enum")
		}
	}

	node := &schemaNode{
		kind:       kind,
		constValue: constValue,
		enumValues: enumValues,
	}
	if kind == schemaKindObject {
		node.properties = properties
		node.required = required
		node.additionalPropertiesMode = additionalMode
		node.additionalPropertiesSchema = additionalSchema
	}
	if kind == schemaKindArray {
		node.items = items
	}
	return node, nil
}

func compileProperties(raw interface{}, path string) (map[string]*schemaNode, error) {
	if raw == nil {
		return nil, nil
	}
	propsMap, ok := asSchemaMap(raw)
	if !ok {
		return nil, unsupportedSchemaError(path, "properties must be an object")
	}
	if len(propsMap) == 0 {
		return nil, nil
	}
	properties := make(map[string]*schemaNode, len(propsMap))
	for key, value := range propsMap {
		childRaw, ok := asSchemaMap(value)
		if !ok {
			return nil, unsupportedSchemaError(path, fmt.Sprintf("property %q must be a schema object", key))
		}
		child, err := compileSchemaNode(childRaw, path+".properties."+key)
		if err != nil {
			return nil, err
		}
		properties[key] = child
	}
	return properties, nil
}

func compileRequired(raw interface{}, path string) (map[string]struct{}, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, true, unsupportedSchemaError(path, "required must be an array of strings")
	}
	if len(items) == 0 {
		return nil, true, nil
	}
	required := make(map[string]struct{}, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			return nil, true, unsupportedSchemaError(path, "required must be an array of strings")
		}
		required[name] = struct{}{}
	}
	return required, true, nil
}

func compileItems(raw interface{}, path string) (*schemaNode, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	itemsSchema, ok := asSchemaMap(raw)
	if !ok {
		return nil, true, unsupportedSchemaError(path, "items must be a schema object")
	}
	items, err := compileSchemaNode(itemsSchema, path+".items")
	if err != nil {
		return nil, true, err
	}
	return items, true, nil
}

func compileAdditionalProperties(raw interface{}, path string) (additionalPropertiesMode, *schemaNode, bool, error) {
	if raw == nil {
		return additionalPropertiesAllowOpen, nil, false, nil
	}
	switch value := raw.(type) {
	case bool:
		if value {
			return additionalPropertiesAllowOpen, nil, true, nil
		}
		return additionalPropertiesForbid, nil, true, nil
	case map[string]interface{}:
		schema, err := compileSchemaNode(value, path+".additionalProperties")
		if err != nil {
			return additionalPropertiesTyped, nil, true, err
		}
		return additionalPropertiesTyped, schema, true, nil
	case JSONSchema:
		schema, err := compileSchemaNode(map[string]interface{}(value), path+".additionalProperties")
		if err != nil {
			return additionalPropertiesTyped, nil, true, err
		}
		return additionalPropertiesTyped, schema, true, nil
	default:
		return additionalPropertiesAllowOpen, nil, true, unsupportedSchemaError(path, "additionalProperties must be bool or schema object")
	}
}

func parseSchemaKind(raw interface{}, path string) (schemaKind, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	switch value := raw.(type) {
	case string:
		switch value {
		case "object":
			return schemaKindObject, true, nil
		case "array":
			return schemaKindArray, true, nil
		case "string":
			return schemaKindString, true, nil
		case "number":
			return schemaKindNumber, true, nil
		case "boolean":
			return schemaKindBoolean, true, nil
		case "null":
			return schemaKindNull, true, nil
		default:
			return 0, true, unsupportedSchemaError(path, fmt.Sprintf("unsupported type %q", value))
		}
	case []interface{}:
		return 0, true, unsupportedSchemaError(path, "type as array is not supported")
	default:
		return 0, true, unsupportedSchemaError(path, "type must be a string")
	}
}

func compileConstValue(raw map[string]interface{}, path string) (*scalarLiteral, bool, error) {
	constRaw, hasConst := raw["const"]
	if !hasConst {
		return nil, false, nil
	}
	value, ok := scalarLiteralFromInterface(constRaw)
	if !ok {
		return nil, true, unsupportedSchemaError(path, "const must be scalar (string, number, boolean, null)")
	}
	return &value, true, nil
}

func compileEnumValues(raw map[string]interface{}, path string) (*scalarLiteralSet, bool, error) {
	enumRaw, hasEnum := raw["enum"]
	if !hasEnum {
		return nil, false, nil
	}
	values, ok := asInterfaceSlice(enumRaw)
	if !ok {
		return nil, true, unsupportedSchemaError(path, "enum must be an array")
	}
	if len(values) == 0 {
		return nil, true, unsupportedSchemaError(path, "enum must not be empty")
	}
	enumValues := make([]scalarLiteral, 0, len(values))
	var enumKind scalarLiteralKind
	for i, rawValue := range values {
		switch rawValue.(type) {
		case []interface{}:
			return nil, true, unsupportedSchemaError(path, "enum of arrays is not supported")
		case map[string]interface{}, JSONSchema:
			return nil, true, unsupportedSchemaError(path, "enum of objects is not supported")
		}
		parsed, ok := scalarLiteralFromInterface(rawValue)
		if !ok {
			return nil, true, unsupportedSchemaError(path, "enum values must be scalar (string, number, boolean, null)")
		}
		if i == 0 {
			enumKind = parsed.kind
		} else if parsed.kind != enumKind {
			return nil, true, unsupportedSchemaError(path, "mixed-type enum is not supported")
		}
		enumValues = append(enumValues, parsed)
	}
	return scalarLiteralSetFromValues(enumValues), true, nil
}

func asInterfaceSlice(raw interface{}) ([]interface{}, bool) {
	switch value := raw.(type) {
	case []interface{}:
		return value, true
	}
	sliceValue := reflect.ValueOf(raw)
	if !sliceValue.IsValid() || sliceValue.Kind() != reflect.Slice {
		return nil, false
	}
	values := make([]interface{}, sliceValue.Len())
	for i := 0; i < sliceValue.Len(); i++ {
		values[i] = sliceValue.Index(i).Interface()
	}
	return values, true
}

func asSchemaMap(raw interface{}) (map[string]interface{}, bool) {
	switch value := raw.(type) {
	case map[string]interface{}:
		return value, true
	case JSONSchema:
		return map[string]interface{}(value), true
	default:
		return nil, false
	}
}

func unsupportedSchemaError(path, message string) *StaticError {
	return newStaticError(staticErrUnsupportedSchema, "", 0, path+": "+message)
}
