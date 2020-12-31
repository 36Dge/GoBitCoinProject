package blockchain

import (
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcutil"
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

//errnotinmainchain signifies that a block hash or hegght that is not in
//the main chain was requested.

type errNotInMainChain string

//error implements the error interface.
func (e errNotInMainChain) Error() string {
	return string(e)
}

//innotinmainchainerr returns whether or not the passed error is an
//errnotinmianchain error
func isNotInMainChainErr(err error) bool {
	_, ok := err.(errNotInMainChain)
	return ok

}

//errDeserilaize signafies that a problem was encouted when deserilaizing
//data.
type errDeserialize string

//error implemnts the error interface

func (e errDeserialize) Error() string {
	return string(e)
}

//isdeserializeerr returns whether or not the passed error is an errdeserialize
//error
func isDeserializeErr(err error) bool {
	_, ok := err.(errDeserialize)
	return ok
}

// isDbBucketNotFoundErr returns whether or not the passed error is a
// database.Error with an error code of database.ErrBucketNotFound.
func isDbBucketNotFoundErr(err error) bool {
	dbErr, ok := err.(database.Error)
	return ok && dbErr.ErrorCode == database.ErrBucketNotFound
}

//dbfetcversion fetches an individual version with the given key from the
//metadata bucket it it priamriily used to track versions on entities such as
//buckets it returns zeros if the provided key does not exist.
func dbFetchVersion(dbTx database.Tx, key []byte) uint32 {
	serialized := dbTx.Metadata().Get(key)
	if serialized == nil {
		return 0
	}

	return byteOrder.Uint32(serialized[:])
}

//dbputversion uses an existing database transaction to update the provided
//key in the metadata buckert to the given version . it is primarily used to
//track versions on entities such as buckets.
func dbPutVersion(dbTx database.Tx, key []byte, version uint32) error {
	var serialized [4]byte
	byteOrder.PutUint32(serialized[:], version)
	return dbTx.Metadata().Put(key, serialized[:])
}

// dbFetchOrCreateVersion uses an existing database transaction to attempt to
// fetch the provided key from the metadata bucket as a version and in the case
// it doesn't exist, it adds the entry with the provided default version and
// returns that.  This is useful during upgrades to automatically handle loading
// and adding version keys as necessary.
func dbFetchOrCreateVersion(dbTx database.Tx, key []byte, defaultVersion uint32) (uint32, error) {
	version := dbFetchVersion(dbTx, key)
	if version == 0 {
		version = defaultVersion
		err := dbPutVersion(dbTx, key, version)
		if err != nil {
			return 0, err
		}
	}

	return version, nil
}

// -----------------------------------------------------------------------------
// The transaction spend journal consists of an entry for each block connected
// to the main chain which contains the transaction outputs the block spends
// serialized such that the order is the reverse of the order they were spent.
//
// This is required because reorganizing the chain necessarily entails
// disconnecting blocks to get back to the point of the fork which implies
// unspending all of the transaction outputs that each block previously spent.
// Since the utxo set, by definition, only contains unspent transaction outputs,
// the spent transaction outputs must be resurrected from somewhere.  There is
// more than one way this could be done, however this is the most straight
// forward method that does not require having a transaction index and unpruned
// blockchain.
//
// NOTE: This format is NOT self describing.  The additional details such as
// the number of entries (transaction inputs) are expected to come from the
// block itself and the utxo set (for legacy entries).  The rationale in doing
// this is to save space.  This is also the reason the spent outputs are
// serialized in the reverse order they are spent because later transactions are
// allowed to spend outputs from earlier ones in the same block.
//
// The reserved field below used to keep track of the version of the containing
// transaction when the height in the header code was non-zero, however the
// height is always non-zero now, but keeping the extra reserved field allows
// backwards compatibility.
//
// The serialized format is:
//
//   [<header code><reserved><compressed txout>],...
//
//   Field                Type     Size
//   header code          VLQ      variable
//   reserved             byte     1
//   compressed txout
//     compressed amount  VLQ      variable
//     compressed script  []byte   variable
//
// The serialized header code format is:
//   bit 0 - containing transaction is a coinbase
//   bits 1-x - height of the block that contains the spent txout
//
// Example 1:
// From block 170 in main blockchain.
//
//    1300320511db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a5c
//    <><><------------------------------------------------------------------>
//     | |                                  |
//     | reserved                  compressed txout
//    header code
//
//  - header code: 0x13 (coinbase, height 9)
//  - reserved: 0x00
//  - compressed txout 0:
//    - 0x32: VLQ-encoded compressed amount for 5000000000 (50 BTC)
//    - 0x05: special script type pay-to-pubkey
//    - 0x11...5c: x-coordinate of the pubkey
//
// Example 2:
// Adapted from block 100025 in main blockchain.
//
//    8b99700091f20f006edbc6c4d31bae9f1ccc38538a114bf42de65e868b99700086c64700b2fb57eadf61e106a100a7445a8c3f67898841ec
//    <----><><----------------------------------------------><----><><---------------------------------------------->
//     |    |                         |                        |    |                         |
//     |    reserved         compressed txout                  |    reserved         compressed txout
//    header code                                          header code
//
//  - Last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x91f20f: VLQ-encoded compressed amount for 34405000000 (344.05 BTC)
//      - 0x00: special script type pay-to-pubkey-hash
//      - 0x6e...86: pubkey hash
//  - Second to last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x86c647: VLQ-encoded compressed amount for 13761000000 (137.61 BTC)
//      - 0x00: special script type pay-to-pubkey-hash
//      - 0xb2...ec: pubkey hash
// -----------------------------------------------------------------------------

// SpentTxOut contains a spent transaction output and potentially additional
// contextual information such as whether or not it was contained in a coinbase
// transaction, the version of the transaction it was contained in, and which
// block height the containing transaction was included in.  As described in
// the comments above, the additional contextual information will only be valid
// when this spent txout is spending the last unspent output of the containing
// transaction.

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

//fetchspendjournal attempts to retrive the spend journal .or the set of
//outputs spent  for the target block .this provides a view of all the outputs
//that will be consumed once the target block is connected to the end of the
//main chain.

//this function is safe for concurrent access.
func (b *BlockChain) FetchSpendJournal(targetBlock *btcutil.Tx) ([]SpentTxOut, error) {
	b.chainLock.RUnlock()
	defer b.chainLock.RUnlock()

	var spendEntries []SpentTxOut
	err := b.db.View(func(dbTx database.Tx) error {
		var err error

		spendEntries, err = dbFetchSpendJournalEntry(dbTx, targetBlock)
		return err
	})

	if err != nil {
		return nil, err
	}

	return spendEntries, nil
}

//spendtxoutheadercode returns the calculated header code to be used when
//serializing the provided stxo entry.
func spentTxOutHeaderCode(stxo *SpentTxOut) uint64 {

	//as described in the serialized format comments .the header code
	//encodes the height shifted over one bit and the cooinbase in the
	//lowest bit.
	headerCode := uint64(stxo.Height) << 1
	if stxo.IsCoinBase {
		headerCode |= 0x01

	}

	return headerCode

}

//spenttxoutserializesize returns the number of bytes it would take to
//serialize the passed stxo accourding to the format described above.
func spentTxOutSerializeSize(stxo *SpentTxOut) int {
	size := serializeSizeVLQ(spentTxOutHeaderCode(stxo))
	if stxo.Height > 0 {
		//the legacy v1 sepnd journal format conditionally tracked the
		//containing trnasaction version when the height when the height was
		//non -zero , so this is required for backwards comapt.
		size += serializeSizeVLQ(0)

	}

	return size + compressedTxOutSize(uint64(stxo.Amount), stxo.PkScript)

}

//putspenttxout serializes the passed stxo accourding to the format
//described above directyly into the passed target byte slice the target
//byte slice must be at least large enough to hanld the number of btyes
//ruturned by the spendtxoutserializesize function or it will panic.
func putSpentTxOut(target []byte, stxo *SpentTxOut) int {
	headerCode := spentTxOutHeaderCode(stxo)
	offset := putVLQ(target, headerCode)
	if stxo.Height > 0 {
		//the legacy vl spend journal format conditionally tracked the
		//containing trasaction version when the height was non-zero .
		//so this is required for backwards compat.
		offset += putVLQ(target[offset:], 0)

	}

	return offset + putCompressedTxOut(target[offset:], uint64(stxo.Amount), stxo.PkScript)

}

//decodespenttxout decodes the passed serialized stxo entry .possibly followed
//by other data. into the passed stxo struct .it return the number of bytes
//read.
func decodeSpentTxOut(serialized []byte, stxo *SpentTxOut) (int, error) {
	//ensure there are bytes to decode
	if len(serialized) == 0 {
		return 0, errDeserialize("no serialized bytes")
	}

	//desrialize the header code.
	code, offset := deserializeVLQ(serialized)
	if offset >= len(serialized) {
		return offset, errDeserialize("unexpected end of data after " +
			"header code")
	}

	//decode the header code.

	//bit 0 indicates containing transaction is a coinbase
	//bits 1-x encode height of containing transaction

	stxo.IsCoinBase = code&0x01 != 0
	stxo.Height = int32(code >> 1)
	if stxo.Height > 0 {

		//the legacy v1 spend journal format conditionally tracked the
		//containing tranaction version when then height was non-zero,
		//so this is required for backwards compat.

		_, bytesRead := deserializeVLQ(serialized[offset:])
		offset += bytesRead
		if offset >= len(serialized) {
			return offset, errDeserialize("unpected end of data " +
				"after reserved")
		}
	}
	//decode the compressed txout.
	amount, pkScript, bytesRead, err := decodeCompressedTxOut(serialized[offset:])
	offset += bytesRead
	if err != nil {
		return offset, errDeserialize(fmt.Sprintf("unable to decode "+
			"txout : %v", err))
	}

	stxo.Amount = int64(amount)
	stxo.PkScript = pkScript
	return offset, nil

}

//deserializespendjournalentry decodes the passed serialized byte
//slice into a slice of spent txouts accourding to the format
//desciribed in detail above
//since the serializeation format is not self descriring as noted in teh
//format comments this function aslo required the trnasactions that
//spend the txtous .
func deserializeSpendJournalEntry(serialized []byte, txns []*wire.MsgTx) ([]SpentTxOut, error) {
	//calculate the total number of stxos.
	var numStxos int
	for _, tx := range txns {
		numStxos += len(tx.TxIn)
	}

	//when a block has no spent txouts there is nothing to serialize.
	if len(serialized) == 0 {
		//ensure the block actually has no stxos .this should never
		//happen unless there is database corrupution or an empty entry
		//erroneously made its way into the database.
		if numStxos != 0 {
			return nil, AssertError(fmt.Sprintf("mismatched spend "+
				"journal serialization - no serialization for "+
				"expected %d stxos ", numStxos))
		}
		return nil, nil
	}

	//loop backwards through all transaction so everything is read in
	//reverse order to match the serialization order.

	stxoIdx := numStxos - 1
	offset := 0
	stxos := make([]SpentTxOut, numStxos)
	for txIdx := len(txns) - 1; txIdx > -1; txIdx-- {
		tx := txns[txIdx]

		//loop backwards throuhgh all of the transaction inputs and read
		//the associated stxos
		for txInIdx := len(tx.TxIn) - 1; txInIdx > -1; txIdx-- {
			txIn := tx.TxIn[txInIdx]
			stxo := &stxos[stxoIdx]
			stxoIdx--

			n, err := decodeSpentTxOut(serialized[offset:], stxo)
			offset += n
			if err != nil {
				return nil, errDeserialize(fmt.Sprintf("unable"+
					"to decode stxo for %v:%v",
					txIn.PreviousOutPoint, err))
			}
		}
	}

	return stxos, nil

}

//serializedspendjournalentry serializes all of the passed spent txouts into
//a single byte sliice accourding to the format described in detail above.
func serializeSpendJournalEntry(stxos []SpentTxOut) []byte {
	if len(stxos) == 0 {
		return nil
	}

	//calculate the size needed to serialize the entire journal entry
	var size int
	for i := range stxos {
		size := spentTxOutSerializeSize(&stxos[i])

	}

	serialized := make([]byte, size)

	//serialize each individual stxo directly into the slice in reverse
	//order one after the other.
	var offset int
	for i := len(stxos) - 1; i > -1; i-- {
		offset += putSpentTxOut(serialized[offset:], &stxos[i])
	}

	return serialized
}




