package jmespath

// JSONSchema is a minimal JSON schema representation accepted by CompileSchema.
type JSONSchema map[string]interface{}

type schemaKind uint8

const (
	schemaKindObject schemaKind = iota + 1
	schemaKindArray
	schemaKindString
	schemaKindNumber
	schemaKindBoolean
	schemaKindNull
)

type stringFormat uint8

const (
	stringFormatNone stringFormat = iota
	stringFormatDate
)

type additionalPropertiesMode uint8

const (
	additionalPropertiesForbid additionalPropertiesMode = iota
	additionalPropertiesAllowOpen
	additionalPropertiesTyped
)

type schemaNode struct {
	kind                       schemaKind
	oneOf                      []*schemaNode
	properties                 map[string]*schemaNode
	required                   map[string]struct{}
	items                      *schemaNode
	additionalPropertiesMode   additionalPropertiesMode
	additionalPropertiesSchema *schemaNode
	constValue                 *scalarLiteral
	enumValues                 *scalarLiteralSet
	stringFormat               stringFormat
}

// CompiledSchema is an internal IR used by CompileWithCompiledSchema.
type CompiledSchema struct {
	root       *schemaNode
	staticRoot *staticType
}
