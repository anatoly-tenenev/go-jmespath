package jmespath

import "fmt"

const (
	staticErrUnsupportedSchema    = "unsupported_schema"
	staticErrUnknownProperty      = "unknown_property"
	staticErrUnverifiableProperty = "unverifiable_property"
	staticErrInvalidFieldTarget   = "invalid_field_target"
	staticErrInvalidIndexTarget   = "invalid_index_target"
	staticErrInvalidProjection    = "invalid_projection_target"
	staticErrInvalidComparator    = "invalid_comparator_types"
	staticErrUnknownFunction      = "unknown_function"
	staticErrInvalidFuncArity     = "invalid_function_arity"
	staticErrInvalidFuncArgType   = "invalid_function_arg_type"
	staticErrUnverifiableType     = "unverifiable_type"
	staticErrInvalidEnumValue     = "invalid_enum_value"
)

// StaticError reports compile-time schema-aware validation failures.
type StaticError struct {
	Code       string
	Expression string
	Offset     int
	Message    string
}

func (e *StaticError) Error() string {
	if e == nil {
		return ""
	}
	if e.Expression == "" {
		if e.Message == "" {
			return e.Code
		}
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	if e.Message == "" {
		return fmt.Sprintf("%s at offset %d", e.Code, e.Offset)
	}
	return fmt.Sprintf("%s at offset %d: %s", e.Code, e.Offset, e.Message)
}

func newStaticError(code, expression string, offset int, message string) *StaticError {
	return &StaticError{
		Code:       code,
		Expression: expression,
		Offset:     offset,
		Message:    message,
	}
}
