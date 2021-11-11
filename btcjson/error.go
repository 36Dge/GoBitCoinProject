package btcjson

import "fmt"

//errorcode identifies a kind of error,these error codes are not used for
//json_prc response errors.
type ErrorCode int

//these constants are used to identify a specific ruleError.
const (
	// ErrDuplicateMethod indicates a command with the specified method
	// already exists.
	ErrDuplicateMethod ErrorCode = iota

	// ErrInvalidUsageFlags indicates one or more unrecognized flag bits
	// were specified.
	ErrInvalidUsageFlags

	// ErrInvalidType indicates a type was passed that is not the required
	// type.
	ErrInvalidType

	// ErrEmbeddedType indicates the provided command struct contains an
	// embedded type which is not not supported.
	ErrEmbeddedType

	// ErrUnexportedField indiciates the provided command struct contains an
	// unexported field which is not supported.
	ErrUnexportedField

	// ErrUnsupportedFieldType indicates the type of a field in the provided
	// command struct is not one of the supported types.
	ErrUnsupportedFieldType

	// ErrNonOptionalField indicates a non-optional field was specified
	// after an optional field.
	ErrNonOptionalField

	// ErrNonOptionalDefault indicates a 'jsonrpcdefault' struct tag was
	// specified for a non-optional field.
	ErrNonOptionalDefault

	// ErrMismatchedDefault indicates a 'jsonrpcdefault' struct tag contains
	// a value that doesn't match the type of the field.
	ErrMismatchedDefault

	// ErrUnregisteredMethod indicates a method was specified that has not
	// been registered.
	ErrUnregisteredMethod

	// ErrMissingDescription indicates a description required to generate
	// help is missing.
	ErrMissingDescription

	// ErrNumParams inidcates the number of params supplied do not
	// match the requirements of the associated command.
	ErrNumParams

	// numErrorCodes is the maximum error code number used in tests.
	numErrorCodes
)

//map of errorcode vaules back to their constanss name for pretty printing
var errorCodeStrings = map[ErrorCode]string{
	ErrDuplicateMethod:      "ErrDuplicateMethod",
	ErrInvalidUsageFlags:    "ErrInvalidUsageFlags",
	ErrInvalidType:          "ErrInvalidType",
	ErrEmbeddedType:         "ErrEmbeddedType",
	ErrUnexportedField:      "ErrUnexportedField",
	ErrUnsupportedFieldType: "ErrUnsupportedFieldType",
	ErrNonOptionalField:     "ErrNonOptionalField",
	ErrNonOptionalDefault:   "ErrNonOptionalDefault",
	ErrMismatchedDefault:    "ErrMismatchedDefault",
	ErrUnregisteredMethod:   "ErrUnregisteredMethod",
	ErrMissingDescription:   "ErrMissingDescription",
	ErrNumParams:            "ErrNumParams",
}

//string returns the errorcode as a human-readbale name.
func (e ErrorCode) String() string {
	if s := errorCodeStrings[e]; s != "" {
		return s
	}
	return fmt.Sprintf("unknown ErrorCode (%d)", int(e))
}

type Error struct {
	ErrorCode   ErrorCode
	Description string
}

//error satisfies the error interface and prints human-readbale errors.

func (e Error) Error() string {
	return e.Description
}

//makeerror create an error given a set of arguments
func makeError(c ErrorCode, desc string) Error {
	return Error{ErrorCode: c, Description: desc}
}

//over
