package database

//errorcode indentifies a  kind of error.
type ErrorCode int

//these constants are used to indentify a specific database error.
const (

	// **************************************
	// Errors related to driver registration.
	// **************************************

	// ErrDbTypeRegistered indicates two different database drivers
	// attempt to register with the name database type.
	ErrDbTypeRegistered ErrorCode = iota

	// *************************************
	// Errors related to database functions.
	// *************************************

	// ErrDbUnknownType indicates there is no driver registered for
	// the specified database type.
	ErrDbUnknownType

	// ErrDbDoesNotExist indicates open is called for a database that
	// does not exist.
	ErrDbDoesNotExist

	// ErrDbExists indicates create is called for a database that
	// already exists.
	ErrDbExists

	// ErrDbNotOpen indicates a database instance is accessed before
	// it is opened or after it is closed.
	ErrDbNotOpen

	// ErrDbAlreadyOpen indicates open was called on a database that
	// is already open.
	ErrDbAlreadyOpen

	// ErrInvalid indicates the specified database is not valid.
	ErrInvalid

	// ErrCorruption indicates a checksum failure occurred which invariably
	// means the database is corrupt.
	ErrCorruption

	// ****************************************
	// Errors related to database transactions.
	// ****************************************

	// ErrTxClosed indicates an attempt was made to commit or rollback a
	// transaction that has already had one of those operations performed.
	ErrTxClosed

	// ErrTxNotWritable indicates an operation that requires write access to
	// the database was attempted against a read-only transaction.
	ErrTxNotWritable

	// **************************************
	// Errors related to metadata operations.
	// **************************************

	// ErrBucketNotFound indicates an attempt to access a bucket that has
	// not been created yet.
	ErrBucketNotFound

	// ErrBucketExists indicates an attempt to create a bucket that already
	// exists.
	ErrBucketExists

	// ErrBucketNameRequired indicates an attempt to create a bucket with a
	// blank name.
	ErrBucketNameRequired

	// ErrKeyRequired indicates at attempt to insert a zero-length key.
	ErrKeyRequired

	// ErrKeyTooLarge indicates an attmempt to insert a key that is larger
	// than the max allowed key size.  The max key size depends on the
	// specific backend driver being used.  As a general rule, key sizes
	// should be relatively, so this should rarely be an issue.
	ErrKeyTooLarge

	// ErrValueTooLarge indicates an attmpt to insert a value that is larger
	// than max allowed value size.  The max key size depends on the
	// specific backend driver being used.
	ErrValueTooLarge

	// ErrIncompatibleValue indicates the value in question is invalid for
	// the specific requested operation.  For example, trying create or
	// delete a bucket with an existing non-bucket key, attempting to create
	// or delete a non-bucket key with an existing bucket key, or trying to
	// delete a value via a cursor when it points to a nested bucket.
	ErrIncompatibleValue

	// ***************************************
	// Errors related to block I/O operations.
	// ***************************************

	// ErrBlockNotFound indicates a block with the provided hash does not
	// exist in the database.
	ErrBlockNotFound

	// ErrBlockExists indicates a block with the provided hash already
	// exists in the database.
	ErrBlockExists

	// ErrBlockRegionInvalid indicates a region that exceeds the bounds of
	// the specified block was requested.  When the hash provided by the
	// region does not correspond to an existing block, the error will be
	// ErrBlockNotFound instead.
	ErrBlockRegionInvalid

	// ***********************************
	// Support for driver-specific errors.
	// ***********************************

	// ErrDriverSpecific indicates the Err field is a driver-specific error.
	// This provides a mechanism for drivers to plug-in their own custom
	// errors for any situations which aren't already covered by the error
	// codes provided by this package.
	ErrDriverSpecific

	// numErrorCodes is the maximum error code number used in tests.
	numErrorCodes
)

// map of errorcode values back to their constant names for pretty printing .
var errorCodeStrings = map[ErrorCode]string{
	ErrDbTypeRegistered:   "ErrDbTypeRegistered",
	ErrDbUnknownType:      "ErrDbUnknownType",
	ErrDbDoesNotExist:     "ErrDbDoesNotExist",
	ErrDbExists:           "ErrDbExists",
	ErrDbNotOpen:          "ErrDbNotOpen",
	ErrDbAlreadyOpen:      "ErrDbAlreadyOpen",
	ErrInvalid:            "ErrInvalid",
	ErrCorruption:         "ErrCorruption",
	ErrTxClosed:           "ErrTxClosed",
	ErrTxNotWritable:      "ErrTxNotWritable",
	ErrBucketNotFound:     "ErrBucketNotFound",
	ErrBucketExists:       "ErrBucketExists",
	ErrBucketNameRequired: "ErrBucketNameRequired",
	ErrKeyRequired:        "ErrKeyRequired",
	ErrKeyTooLarge:        "ErrKeyTooLarge",
	ErrValueTooLarge:      "ErrValueTooLarge",
	ErrIncompatibleValue:  "ErrIncompatibleValue",
	ErrBlockNotFound:      "ErrBlockNotFound",
	ErrBlockExists:        "ErrBlockExists",
	ErrBlockRegionInvalid: "ErrBlockRegionInvalid",
	ErrDriverSpecific:     "ErrDriverSpecific",
}

type Error struct {
	ErrorCode   ErrorCode //descirpe the kind of error
	Description string    //human readable description of the inssure
	Err         error     //underlaying error
}

//error satisfies the error interface and prints human-readable errors.
func (e Error) Error() string {
	if e.Err != nil {
		return e.Description + ":" + e.Err.Error()
	}

	return e.Description
}

//makeerror creates an error given s set of arguments .the error code must
//be one of the error codes provided by this package.
func makeError(c ErrorCode, desc string, err error) Error {
	return Error{ErrorCode: c, Description: desc, Err: err}
}
