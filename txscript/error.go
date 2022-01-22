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

	//ErrInvalidFlags is returned when the passed flags to newengine
	//contain an invalid combication
	ErrInvalidFlags

	//errinvalidindex is returned when an out-of-bounds ins passed to
	//a function
	ErrInvalidIndex

	//errunsupporetedaddress is returned when a concrete type that
	//implements a btcutil.address is not a supported type.
	ErrUnsupportedAddress
	//errnotmultisigscript is returned from calcmultisigstats when the
	//provided script is not a multisig script .
	errNotMultisigScript

	//errtoomanyrequiredsigs isreturned from multisigscript when the
	//specified number of required signatures is large than the number of
	//provided public keys.
	ErrTooManyRequiredSigs
	//errtoomuchnulldata is retuned from nulldatascript when the length of
	//the provided data exceeds maxdatacarrierSize.
	ErrTooMuchNullData

	//-------------
	// Failure related to final execution state.
	//-------------

	//erreralyreturn is retuned when op_return is execcuted in the script
	ErrEarlyReturn

	//erremptystatck is returned when the script evaluated without error.
	//but terminated with an empty top stack element.
	ErrEmptyStack















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