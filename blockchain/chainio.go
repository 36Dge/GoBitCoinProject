package blockchain

import (
	"BtcoinProject/wire"
	"encoding/binary"
)

const (

	//blockhdrsize is the size of a block header.this is simply the
	//constant from wire and is only provided here for convenience since
	//wire.maxblockheaderpayload is quite long
	blockHdrSize = wire.MaxBlockHeaderPayload

	//latestutxosetbuctversion is the current version of the utxo set
	//bucket that is used to track fall unspent outputs.
	latestUtxoSetBucketVersion = 2

	//latestspendjurnalbucketversion is the curret version of the spend
	//journal bucket that is used to tranck all spent transaction for use
	//in reorgs.
	latestSpendJournalBucketVersion = 1
)

var (

	//blockindexbuckerName is the name of the db bucket used to house to the
	//block headers and contextual information.
	blockIndexBucketName = []byte("blockheaderidx")

	//hashindexbucketname is the name of the db bucket used to house to the
	//block hash -->block height index.
	hashIndexBucketName = []byte("hashidx")

	//heightindexbucketName is the name of the db bucket used to house to
	//the block height - > block hash index.
	heightIndexBucketName = []byte("heightidx")

	//chainstatekeyName is the name of the db key used to store the best
	//chain satte.
	chainStateKeyName = []byte("chainstate")

	//spendjournalversionkeyname is the name of the db key used to store
	//the version of the spend journal currently in the database
	spendJournalVersionKeyName = []byte("spendjournalversion")

	//spendjournalbucketname is the name of the db bucket used to house
	//trnasactions outputs that are spent in each block
	spendJournalBucketName = []byte("spendjournal")

	//utxosetversionkeyname is the name of the db key used to store the
	//version of the utxo set currently in the database.
	utxoSetVersionKeyName = []byte("utxosetversion")

	//utxosetbucketname is the name of the db bucket used to house
	//the unspent trnasaction output set .
	utxoSetBucketName = []byte("utxosetv2")

	//byteorder is the preferenced order used for serialing numeric
	//fields for storage in the database.
	byteOrder = binary.LittleEndian
)



type SpentTxOut struct {
	// Amount is the amount of the output.
	Amount int64

	// PkScipt is the the public key script for the output.
	PkScript []byte

	// Height is the height of the the block containing the creating tx.
	Height int32

	// Denotes if the creating tx is a coinbase.
	IsCoinBase bool
}
