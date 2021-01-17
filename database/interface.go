package database

import (
	"BtcoinProject/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
)


type Cursor interface {

	// Bucket returns the bucket the cursor was created for.
	Bucket() Bucket

	// Delete removes the current key/value pair the cursor is at without
	// invalidating the cursor.
	//
	// The interface contract guarantees at least the following errors will
	// be returned (other implementation-specific errors are possible):
	//   - ErrIncompatibleValue if attempted when the cursor points to a
	//     nested bucket
	//   - ErrTxNotWritable if attempted against a read-only transaction
	//   - ErrTxClosed if the transaction has already been closed
	Delete() error

	// First positions the cursor at the first key/value pair and returns
	// whether or not the pair exists.
	First() bool

	// Last positions the cursor at the last key/value pair and returns
	// whether or not the pair exists.
	Last() bool

	// Next moves the cursor one key/value pair forward and returns whether
	// or not the pair exists.
	Next() bool

	// Prev moves the cursor one key/value pair backward and returns whether
	// or not the pair exists.
	Prev() bool

	// Seek positions the cursor at the first key/value pair that is greater
	// than or equal to the passed seek key.  Returns whether or not the
	// pair exists.
	Seek(seek []byte) bool

	// Key returns the current key the cursor is pointing to.
	Key() []byte

	// Value returns the current value the cursor is pointing to.  This will
	// be nil for nested buckets.
	Value() []byte



}

type Tx interface {
	Metadata() Bucket
	StoreBlock(block *btcutil.Block) error
	HasBlock(hash *chainhash.Hash) (bool, error)
	HasBlocks(hashes []chainhash.Hash) ([]bool, error)
	FetchBlockHeader(hash *chainhash.Hash) ([]byte, error)
	FetchBlockHeaders(hashes []chainhash.Hash) ([][]byte, error)
	FetchBlock(hash *chainhash.Hash) ([]byte, error)
	FetchBlocks(hashes []chainhash.Hash) ([][]byte, error)
	FetchBlockRegion(region *BlockRegion) ([]byte, error)
	FetchBlockRegions(regions []BlockRegion) ([][]byte, error)
	Commit() error
	Rollback() error
}

type DB interface {
	Type() string
	Begin(writable bool) (Tx, error)
	View(fn func(tx Tx) error) error
	Update(fn func(tx Tx) error) error
	Close() error
}

type Bucket interface {
	Bucket(key []byte) Bucket

	CreateBucket(key []byte) (Bucket, error)

	CreateBucketIfNotExists(key []byte) (Bucket, error)

	DeleteBucket(key []byte) error

	ForEach(func(k, v []byte) error) error

	ForEachBucket(func(k []byte) error) error

	Cursor() Cursor

	Writable() bool

	Put(key, value []byte) error

	Get(key []byte) []byte

	Delete(key []byte) error
}
