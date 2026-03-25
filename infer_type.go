package jmespath

// TypeMask is a compact bitmask representation of possible result types.
type TypeMask uint8

// Result type bits used by InferredType.Mask.
const (
	TypeObject TypeMask = 1 << iota
	TypeArray
	TypeString
	TypeNumber
	TypeBoolean
	TypeNull
)

// TypeAny represents a value that may be any supported JMESPath type.
const TypeAny = TypeObject | TypeArray | TypeString | TypeNumber | TypeBoolean | TypeNull

// InferredType is a schema-aware static type inferred for an expression.
type InferredType struct {
	Mask       TypeMask
	Item       *InferredType
	Properties map[string]*InferredType
	OpenObject bool
	Const      interface{}
	Enum       []interface{}
}

// IsBoolean reports whether the inferred type is exactly boolean.
func (t *InferredType) IsBoolean() bool {
	return t.isExactly(TypeBoolean)
}

// IsNumber reports whether the inferred type is exactly number.
func (t *InferredType) IsNumber() bool {
	return t.isExactly(TypeNumber)
}

// IsString reports whether the inferred type is exactly string.
func (t *InferredType) IsString() bool {
	return t.isExactly(TypeString)
}

// IsNull reports whether the inferred type is exactly null.
func (t *InferredType) IsNull() bool {
	return t.isExactly(TypeNull)
}

// IsArray reports whether the inferred type is exactly array.
func (t *InferredType) IsArray() bool {
	return t.isExactly(TypeArray)
}

// IsObject reports whether the inferred type is exactly object.
func (t *InferredType) IsObject() bool {
	return t.isExactly(TypeObject)
}

// MayBeBoolean reports whether boolean is one of the possible inferred types.
func (t *InferredType) MayBeBoolean() bool {
	return t.mayContain(TypeBoolean)
}

// MayBeNumber reports whether number is one of the possible inferred types.
func (t *InferredType) MayBeNumber() bool {
	return t.mayContain(TypeNumber)
}

// MayBeString reports whether string is one of the possible inferred types.
func (t *InferredType) MayBeString() bool {
	return t.mayContain(TypeString)
}

// MayBeNull reports whether null is one of the possible inferred types.
func (t *InferredType) MayBeNull() bool {
	return t.mayContain(TypeNull)
}

// MayBeArray reports whether array is one of the possible inferred types.
func (t *InferredType) MayBeArray() bool {
	return t.mayContain(TypeArray)
}

// MayBeObject reports whether object is one of the possible inferred types.
func (t *InferredType) MayBeObject() bool {
	return t.mayContain(TypeObject)
}

// IsUnion reports whether the inferred type may be more than one kind.
func (t *InferredType) IsUnion() bool {
	if t == nil {
		return false
	}
	mask := t.Mask
	return mask != 0 && mask&(mask-1) != 0
}

func (t *InferredType) isExactly(mask TypeMask) bool {
	return t != nil && t.Mask == mask
}

func (t *InferredType) mayContain(mask TypeMask) bool {
	return t != nil && t.Mask&mask != 0
}

type staticToInferredConverter struct {
	cache map[*staticType]*InferredType
}

func inferredTypeFromStatic(typ *staticType) *InferredType {
	current := normalizeStaticType(typ)
	if current.mask == staticMaskAny && current.array == nil && current.object == nil && current.constValue == nil && current.enumValues == nil {
		return inferredUnknownAnyType()
	}
	if current.mask&(staticMaskObject|staticMaskArray) == 0 {
		return inferredScalarStaticType(current)
	}
	converter := staticToInferredConverter{
		cache: make(map[*staticType]*InferredType),
	}
	return converter.convert(current)
}

func inferredScalarStaticType(typ *staticType) *InferredType {
	inferred := &InferredType{Mask: inferredMaskFromStaticMask(typ.mask)}
	if typ.constValue != nil {
		inferred.Const = scalarLiteralToInterface(*typ.constValue)
	}
	if typ.enumValues != nil && len(typ.enumValues.values) > 0 {
		inferred.Enum = scalarLiteralSetToInterfaces(typ.enumValues)
	}
	return inferred
}

func (c *staticToInferredConverter) convert(typ *staticType) *InferredType {
	current := normalizeStaticType(typ)
	if current.mask == staticMaskAny && current.array == nil && current.object == nil && current.constValue == nil && current.enumValues == nil {
		return inferredUnknownAnyType()
	}
	if cached, exists := c.cache[current]; exists {
		return cached
	}
	inferred := &InferredType{
		Mask: inferredMaskFromStaticMask(current.mask),
	}
	c.cache[current] = inferred

	if current.constValue != nil {
		inferred.Const = scalarLiteralToInterface(*current.constValue)
	}
	if current.enumValues != nil && len(current.enumValues.values) > 0 {
		inferred.Enum = scalarLiteralSetToInterfaces(current.enumValues)
	}

	if current.includes(staticMaskArray) {
		if current.array != nil {
			inferred.Item = c.convert(current.array.itemType())
		} else {
			inferred.Item = inferredUnknownAnyType()
		}
	}

	if current.includes(staticMaskObject) {
		if current.object == nil {
			inferred.OpenObject = true
		} else {
			inferred.OpenObject = current.object.additionalMode != additionalPropertiesForbid
			if len(current.object.properties) > 0 {
				properties := make(map[string]*InferredType, len(current.object.properties))
				for key, value := range current.object.properties {
					properties[key] = c.convert(value)
				}
				inferred.Properties = properties
			}
		}
	}

	return inferred
}

func inferredMaskFromStaticMask(mask staticTypeMask) TypeMask {
	var inferred TypeMask
	if mask&staticMaskObject != 0 {
		inferred |= TypeObject
	}
	if mask&staticMaskArray != 0 {
		inferred |= TypeArray
	}
	if mask&staticMaskString != 0 {
		inferred |= TypeString
	}
	if mask&staticMaskNumber != 0 {
		inferred |= TypeNumber
	}
	if mask&staticMaskBoolean != 0 {
		inferred |= TypeBoolean
	}
	if mask&staticMaskNull != 0 {
		inferred |= TypeNull
	}
	return inferred
}

func scalarLiteralSetToInterfaces(values *scalarLiteralSet) []interface{} {
	if values == nil || len(values.values) == 0 {
		return nil
	}
	result := make([]interface{}, len(values.values))
	for i, value := range values.values {
		result[i] = scalarLiteralToInterface(value)
	}
	return result
}

func scalarLiteralToInterface(value scalarLiteral) interface{} {
	switch value.kind {
	case scalarLiteralString:
		return value.stringValue
	case scalarLiteralNumber:
		return value.numberValue
	case scalarLiteralBoolean:
		return value.boolValue
	case scalarLiteralNull:
		return nil
	default:
		return nil
	}
}

func inferredUnknownAnyType() *InferredType {
	return &InferredType{
		Mask:       TypeAny,
		OpenObject: true,
	}
}
