package blockchain

import "BtcoinProject/wire"





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

