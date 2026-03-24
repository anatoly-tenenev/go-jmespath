package jmespath

import (
	"encoding/json"
	"strconv"
)

type scalarLiteralKind uint8

const (
	scalarLiteralString scalarLiteralKind = iota + 1
	scalarLiteralNumber
	scalarLiteralBoolean
	scalarLiteralNull
)

type scalarLiteral struct {
	kind        scalarLiteralKind
	stringValue string
	numberValue float64
	boolValue   bool
}

type scalarLiteralSet struct {
	values []scalarLiteral
	keys   map[string]struct{}
}

func scalarLiteralFromInterface(raw interface{}) (scalarLiteral, bool) {
	switch value := raw.(type) {
	case nil:
		return scalarLiteral{kind: scalarLiteralNull}, true
	case string:
		return scalarLiteral{kind: scalarLiteralString, stringValue: value}, true
	case bool:
		return scalarLiteral{kind: scalarLiteralBoolean, boolValue: value}, true
	case json.Number:
		number, err := value.Float64()
		if err != nil {
			return scalarLiteral{}, false
		}
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(number)}, true
	case float64:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(value)}, true
	case float32:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case int:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case int8:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case int16:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case int32:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case int64:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case uint:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case uint8:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case uint16:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case uint32:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	case uint64:
		return scalarLiteral{kind: scalarLiteralNumber, numberValue: normalizeScalarNumber(float64(value))}, true
	default:
		return scalarLiteral{}, false
	}
}

func normalizeScalarNumber(value float64) float64 {
	if value == 0 {
		return 0
	}
	return value
}

func (v scalarLiteral) toSchemaKind() schemaKind {
	switch v.kind {
	case scalarLiteralString:
		return schemaKindString
	case scalarLiteralNumber:
		return schemaKindNumber
	case scalarLiteralBoolean:
		return schemaKindBoolean
	case scalarLiteralNull:
		return schemaKindNull
	default:
		return 0
	}
}

func (v scalarLiteral) key() string {
	switch v.kind {
	case scalarLiteralString:
		return "s:" + v.stringValue
	case scalarLiteralNumber:
		return "n:" + strconv.FormatFloat(normalizeScalarNumber(v.numberValue), 'g', -1, 64)
	case scalarLiteralBoolean:
		if v.boolValue {
			return "b:true"
		}
		return "b:false"
	case scalarLiteralNull:
		return "z:null"
	default:
		return ""
	}
}

func (v scalarLiteral) equals(other scalarLiteral) bool {
	if v.kind != other.kind {
		return false
	}
	switch v.kind {
	case scalarLiteralString:
		return v.stringValue == other.stringValue
	case scalarLiteralNumber:
		return normalizeScalarNumber(v.numberValue) == normalizeScalarNumber(other.numberValue)
	case scalarLiteralBoolean:
		return v.boolValue == other.boolValue
	case scalarLiteralNull:
		return true
	default:
		return false
	}
}

func scalarLiteralSetFromValues(values []scalarLiteral) *scalarLiteralSet {
	if len(values) == 0 {
		return nil
	}
	set := &scalarLiteralSet{
		values: make([]scalarLiteral, 0, len(values)),
		keys:   make(map[string]struct{}, len(values)),
	}
	for _, value := range values {
		key := value.key()
		if _, exists := set.keys[key]; exists {
			continue
		}
		set.keys[key] = struct{}{}
		set.values = append(set.values, value)
	}
	return set
}

func (s *scalarLiteralSet) contains(value scalarLiteral) bool {
	if s == nil {
		return false
	}
	_, exists := s.keys[value.key()]
	return exists
}

func (s *scalarLiteralSet) kind() (scalarLiteralKind, bool) {
	if s == nil || len(s.values) == 0 {
		return 0, false
	}
	return s.values[0].kind, true
}

func formatLiteralForMessage(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return "<literal>"
}

func (t *staticType) scalarConstraintKeyword() (string, bool) {
	current := normalizeStaticType(t)
	if current.constValue != nil {
		return "const", true
	}
	if current.enumValues != nil {
		return "enum", true
	}
	return "", false
}

func (t *staticType) allowsScalarLiteral(value scalarLiteral) bool {
	current := normalizeStaticType(t)
	if current.constValue != nil {
		return current.constValue.equals(value)
	}
	if current.enumValues != nil {
		return current.enumValues.contains(value)
	}
	return true
}
