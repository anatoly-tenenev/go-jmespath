package jmespath

import "fmt"

var oneOfSiblingKeywords = map[string]struct{}{
	"type":                 {},
	"format":               {},
	"properties":           {},
	"required":             {},
	"items":                {},
	"additionalProperties": {},
	"const":                {},
	"enum":                 {},
}

func validateOneOfSiblings(raw map[string]interface{}, path string) error {
	for key := range oneOfSiblingKeywords {
		if _, exists := raw[key]; exists {
			return unsupportedSchemaError(path, fmt.Sprintf("oneOf with sibling keyword %q is not supported yet", key))
		}
	}
	return nil
}

func compileOneOf(raw interface{}, path string) ([]*schemaNode, error) {
	items, ok := asInterfaceSlice(raw)
	if !ok {
		return nil, unsupportedSchemaError(path, "oneOf must be a non-empty array of schema objects")
	}
	if len(items) == 0 {
		return nil, unsupportedSchemaError(path, "oneOf must be a non-empty array of schema objects")
	}
	branches := make([]*schemaNode, 0, len(items))
	for i, item := range items {
		childRaw, ok := asSchemaMap(item)
		if !ok {
			return nil, unsupportedSchemaError(path, fmt.Sprintf("oneOf[%d] must be a schema object", i))
		}
		branch, err := compileSchemaNode(childRaw, fmt.Sprintf("%s.oneOf[%d]", path, i))
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

func staticFromOneOf(branches []*schemaNode, cache map[*schemaNode]*staticType) *staticType {
	var merged *staticType
	for _, branch := range branches {
		merged = staticUnion(merged, staticFromSchemaNode(branch, cache))
	}
	return normalizeStaticType(merged)
}

func staticUnionObjectTypes(left, right *staticObjectType) *staticObjectType {
	switch {
	case left == nil || right == nil:
		return nil
	case left == right:
		return left
	}

	names := make(map[string]struct{})
	for name := range left.properties {
		names[name] = struct{}{}
	}
	for name := range right.properties {
		names[name] = struct{}{}
	}

	properties := make(map[string]*staticType, len(names))
	var required map[string]struct{}
	var unverifiable map[string]struct{}
	for name := range names {
		leftType, leftStatus := objectPropertyContribution(left, name)
		rightType, rightStatus := objectPropertyContribution(right, name)
		if leftStatus == propertyContributionUnverifiable || rightStatus == propertyContributionUnverifiable {
			if unverifiable == nil {
				unverifiable = make(map[string]struct{})
			}
			unverifiable[name] = struct{}{}
		}
		merged := staticUnion(leftType, rightType)
		if !staticTypeIsEmpty(merged) {
			properties[name] = merged
		}
		if left.hasGuaranteedProperty(name) && right.hasGuaranteedProperty(name) {
			if required == nil {
				required = make(map[string]struct{})
			}
			required[name] = struct{}{}
		}
	}
	if len(properties) == 0 {
		properties = nil
	}

	additionalMode, additionalSchema, verifiableAdditional := staticUnionAdditionalAccess(left, right)
	if additionalMode == additionalPropertiesAllowOpen {
		verifiableAdditional = false
		additionalSchema = nil
	}
	if additionalMode != additionalPropertiesTyped {
		additionalSchema = nil
	}

	return &staticObjectType{
		properties:                 properties,
		required:                   required,
		unverifiableProperties:     unverifiable,
		additionalMode:             additionalMode,
		additionalSchema:           additionalSchema,
		verifiableAdditionalAccess: verifiableAdditional,
	}
}

type propertyContributionStatus uint8

const (
	propertyContributionKnown propertyContributionStatus = iota
	propertyContributionUnverifiable
)

func objectPropertyContribution(obj *staticObjectType, name string) (*staticType, propertyContributionStatus) {
	if obj == nil {
		return nil, propertyContributionUnverifiable
	}
	if obj.isPropertyUnverifiable(name) {
		return nil, propertyContributionUnverifiable
	}
	if value, exists := obj.properties[name]; exists {
		if obj.isRequired(name) {
			return value, propertyContributionKnown
		}
		return staticNullable(value), propertyContributionKnown
	}
	switch obj.additionalMode {
	case additionalPropertiesForbid:
		return staticNullTypeValue, propertyContributionKnown
	case additionalPropertiesTyped:
		if obj.additionalSchema == nil {
			return nil, propertyContributionUnverifiable
		}
		if obj.verifiableAdditionalAccess {
			return obj.additionalSchema, propertyContributionKnown
		}
		return staticNullable(obj.additionalSchema), propertyContributionKnown
	case additionalPropertiesAllowOpen:
		return nil, propertyContributionUnverifiable
	default:
		return nil, propertyContributionUnverifiable
	}
}

func (t *staticObjectType) hasGuaranteedProperty(name string) bool {
	if t == nil || t.isPropertyUnverifiable(name) {
		return false
	}
	if _, exists := t.properties[name]; !exists {
		return false
	}
	return t.isRequired(name)
}

func staticUnionAdditionalAccess(left, right *staticObjectType) (additionalPropertiesMode, *staticType, bool) {
	leftType, leftStatus, leftHasTyped := additionalPropertyContribution(left)
	rightType, rightStatus, rightHasTyped := additionalPropertyContribution(right)

	if leftStatus == propertyContributionUnverifiable || rightStatus == propertyContributionUnverifiable {
		return additionalPropertiesAllowOpen, nil, false
	}

	if !leftHasTyped && !rightHasTyped {
		return additionalPropertiesForbid, nil, false
	}

	typedContributors := 0
	if leftHasTyped {
		typedContributors++
	}
	if rightHasTyped {
		typedContributors++
	}

	// Unknown-field access is only verifiable when a single branch contributes
	// typed additionalProperties. Multiple contributing branches keep the
	// resulting field shape ambiguous under oneOf.
	return additionalPropertiesTyped, staticUnion(leftType, rightType), typedContributors == 1
}

func additionalPropertyContribution(obj *staticObjectType) (*staticType, propertyContributionStatus, bool) {
	if obj == nil {
		return nil, propertyContributionUnverifiable, false
	}
	switch obj.additionalMode {
	case additionalPropertiesForbid:
		return staticNullTypeValue, propertyContributionKnown, false
	case additionalPropertiesAllowOpen:
		return nil, propertyContributionUnverifiable, false
	case additionalPropertiesTyped:
		if obj.additionalSchema == nil {
			return nil, propertyContributionUnverifiable, true
		}
		if obj.verifiableAdditionalAccess {
			return obj.additionalSchema, propertyContributionKnown, true
		}
		return staticNullable(obj.additionalSchema), propertyContributionKnown, true
	default:
		return nil, propertyContributionUnverifiable, false
	}
}

func collectUnionScalarConstraintValues(left, right *staticType, unionMask staticTypeMask) ([]scalarLiteral, bool, scalarLiteralKind, bool) {
	if unionMask == 0 {
		return nil, false, 0, false
	}
	nonNullMask := unionMask &^ staticMaskNull
	if nonNullMask == 0 {
		return nil, false, scalarLiteralNull, false
	}
	var kind scalarLiteralKind
	switch nonNullMask {
	case staticMaskString:
		kind = scalarLiteralString
	case staticMaskNumber:
		kind = scalarLiteralNumber
	case staticMaskBoolean:
		kind = scalarLiteralBoolean
	default:
		return nil, false, 0, false
	}

	values := make([]scalarLiteral, 0, 4)
	keys := make(map[string]struct{})
	bounded := true
	for _, typ := range []*staticType{left, right} {
		current := normalizeStaticType(typ)
		if current.mask == staticMaskNull {
			continue
		}
		if current.mask&^staticMaskNull != nonNullMask {
			return nil, false, 0, false
		}
		switch {
		case current.constValue != nil:
			if current.constValue.kind != kind {
				return nil, false, 0, false
			}
			appendScalarConstraintValue(&values, keys, *current.constValue)
		case current.enumValues != nil:
			for _, value := range current.enumValues.values {
				if value.kind != kind {
					return nil, false, 0, false
				}
				appendScalarConstraintValue(&values, keys, value)
			}
		default:
			bounded = false
		}
	}
	return values, bounded, kind, true
}

func appendScalarConstraintValue(values *[]scalarLiteral, keys map[string]struct{}, value scalarLiteral) {
	key := value.key()
	if _, exists := keys[key]; exists {
		return
	}
	keys[key] = struct{}{}
	*values = append(*values, value)
}
