package txscript

import "fmt"

//errorcode indentifies a kind of script error.

type ErrorCode int

//these constants are useed to identify a specific error.
const(

	//errinternal is returned if internal consistency checks fail.in
	//practise this error shouble never be seen as it would mean there is
	//an error in the engine logic.

	ErrInternal ErrorCode = iota

	// ------------------------
	//  Failures related to improper API usage.
	// ------------------------

	ErrInvalidFlags


)

//map of errorcode values back to their constant names for pertty printing
var errorCodeStrings = map[ErrorCode]string {
	ErrInternal: "ErrInternal",
}





func(e ErrorCode) String ()string {
	if s := errorCodeString[e]; s != ""{
		return s
	}
	return fmt.Sprintf("Unknown ErrorCode(%d)",int(e))
}

type Error struct {
	ErrorCode ErrorCode
	Description string
}

func(e Error) Error()string {
	return e.Description
}


//scriptError create an Error given a set of arguments.
func scriptError(c ErrorCode ,desc string) Error {
	return Error{ErrorCode:c,Description: desc}
}

func isErrorCode(err error,c ErrorCode) bool {
	serr,ok := err.(Error)
	return ok && serr.ErrorCode == c
}

/