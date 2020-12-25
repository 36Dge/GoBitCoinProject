package database

//errorcode indentifies a  kind of error.
type ErrorCode int


//these constants are used to indentify a specific database error.
const (



















)



















type Error struct {
	ErrorCode   ErrorCode //descirpe the kind of error
	Description string    //human readable description of the inssure
	Err         error     //underlaying error
}

//error satisfies the error interface and prints human-readable errors.
func(e Error) Error() string {
	if e.Err != nil {
		return e.Description + ":" + e.Err.Error()
	}

	return e.Description
}

//makeerror creates an error given s set of arguments .the error code must
//be one of the error codes provided by this package.
func makeError(c ErrorCode,desc string ,err error) Error {
	return Error{ErrorCode: c,Description: desc,Err: err}
}