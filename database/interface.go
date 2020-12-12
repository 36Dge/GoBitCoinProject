package database

import (
	"BtcoinProject/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
)

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
