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

	//errevalfalse is returned when the script evaluated without error but
	//terminated with a false top stack element.
	ErrEvalFalse

	//errscriptUnfinished is returned when checkerrorcondition is called on
	//a script that has not finished execting.
	ErrScriptUnfinished

	//errScriptdone is ...
	// made once all of them have already been executed.  This can happen
	// due to things such as a second call to Execute or calling Step after
	// all opcodes have already been executed.

	ErrInvalidProgramCounter

	// --------------
	//failures related to exceeding maximun allowed limits.
	//---------------

	//errscripttoobig is returned if a script is larger than maxscriptsize

	ErrScriptTooBig

	//errelementtoobig is returned if the size of an element to be pushed
	//to the stack is over maxscriptelementsize.
	ErrElementTooBig


	//errtoomanyoperations is returned is a script has more than
	//maxopsperscript opcodes that do not push data.
	ErrTooManyOperations

	//errstackoverflow is returned when stack and altstack combined depth
	//is over the limit
	ErrStackOverflow

	//errinvalidpubkeycount is returned when the number of public
	//keys specified for a mulsig is either negative or greater than
	//maxpubkeypermultisig.
	ErrInvalidPubKeyCount

	//ErrInvalidsignaturecount is returned when the number of signamtures
	//specified for a multisig either nagative or greater than the number
	//of public keys.
	ErrInvalidSignatureCount

	// ErrNumberTooBig is returned when the argument for an opcode that
	// expects numeric input is larger than the expected maximum number of
	// bytes.  For the most part, opcodes that deal with stack manipulation
	// via offsets, arithmetic, numeric comparison, and boolean logic are
	// those that this applies to.  However, any opcode that expects numeric
	// input may fail with this code.
	ErrNumberTooBig



















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