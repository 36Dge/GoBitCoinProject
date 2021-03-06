package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcutil"
	"math/big"
	"net/http"
	"sync"
	"time"
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

//dbfetchspendjournalentry detchs the spend journal entry for the passed block and
//deserializes it into a slice of spent txout entries.

//note:legacy entries will not have the coinbase flag or height set unless it
//was the final output spend in the containing trnasaction.it is up to the
//caller to handle this properly by looking the information un in the utxo set.
func dbFetchSpendJournalEntry(dbTx database.Tx, block *btcutil.Block) ([]SpentTxOut, error) {
	//exclude the coinbase transaction since it can not spend anything
	spendBucket := dbTx.Metadata().Bucket(spendJournalBucketName)
	serialized := spendBucket.Get(block.Hash()[:])
	blockTxns := block.MsgBlock().Transactions[1:]
	stxos, err := deserializeSpendJournalEntry(serialized, blockTxns)

	if err != nil {
		//ensure any deserialization errors are returned as detabase
		//corruption errors
		if isDeserializeErr(err) {
			return nil, database.Error{
				ErrorCode: database.ErrCorruption,
				Description: fmt.Sprintf("corrupt spend "+
					"information for %v:%v", block.Hash(), err),
			}
		}
		return nil, err
	}
	return stxos, nil
}

// dbPutSpendJournalEntry uses an existing database transaction to update the
// spend journal entry for the given block hash using the provided slice of
// spent txouts.   The spent txouts slice must contain an entry for every txout
// the transactions in the block spend in the order they are spent.
func dbPutSpendJournalEntry(dbTx database.Tx, blockHash *chainhash.Hash, stxos []SpentTxOut) error {
	spendBucket := dbTx.Metadata().Bucket(spendJournalBucketName)
	serialized := serializeSpendJournalEntry(stxos)
	return spendBucket.Put(blockHash[:], serialized)
}

// dbRemoveSpendJournalEntry uses an existing database transaction to remove the
// spend journal entry for the passed block hash.
func dbRemoveSpendJournalEntry(dbTx database.Tx, blockHash *chainhash.Hash) error {
	spendBucket := dbTx.Metadata().Bucket(spendJournalBucketName)
	return spendBucket.Delete(blockHash[:])
}

// -----------------------------------------------------------------------------
// The unspent transaction output (utxo) set consists of an entry for each
// unspent output using a format that is optimized to reduce space using domain
// specific compression algorithms.  This format is a slightly modified version
// of the format used in Bitcoin Core.
//
// Each entry is keyed by an outpoint as specified below.  It is important to
// note that the key encoding uses a VLQ, which employs an MSB encoding so
// iteration of utxos when doing byte-wise comparisons will produce them in
// order.
//
// The serialized key format is:
//   <hash><output index>
//
//   Field                Type             Size
//   hash                 chainhash.Hash   chainhash.HashSize
//   output index         VLQ              variable
//
// The serialized value format is:
//
//   <header code><compressed txout>
//
//   Field                Type     Size
//   header code          VLQ      variable
//   compressed txout
//     compressed amount  VLQ      variable
//     compressed script  []byte   variable
//
// The serialized header code format is:
//   bit 0 - containing transaction is a coinbase
//   bits 1-x - height of the block that contains the unspent txout
//
// Example 1:
// From tx in main blockchain:
// Blk 1, 0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098:0
//
//    03320496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52
//    <><------------------------------------------------------------------>
//     |                                          |
//   header code                         compressed txout
//
//  - header code: 0x03 (coinbase, height 1)
//  - compressed txout:
//    - 0x32: VLQ-encoded compressed amount for 5000000000 (50 BTC)
//    - 0x04: special script type pay-to-pubkey
//    - 0x96...52: x-coordinate of the pubkey
//
// Example 2:
// From tx in main blockchain:
// Blk 113931, 4a16969aa4764dd7507fc1de7f0baa4850a246de90c45e59a3207f9a26b5036f:2
//
//    8cf316800900b8025be1b3efc63b0ad48e7f9f10e87544528d58
//    <----><------------------------------------------>
//      |                             |
//   header code             compressed txout
//
//  - header code: 0x8cf316 (not coinbase, height 113931)
//  - compressed txout:
//    - 0x8009: VLQ-encoded compressed amount for 15000000 (0.15 BTC)
//    - 0x00: special script type pay-to-pubkey-hash
//    - 0xb8...58: pubkey hash
//
// Example 3:
// From tx in main blockchain:
// Blk 338156, 1b02d1c8cfef60a189017b9a420c682cf4a0028175f2f563209e4ff61c8c3620:22
//
//    a8a2588ba5b9e763011dd46a006572d820e448e12d2bbb38640bc718e6
//    <----><-------------------------------------------------->
//      |                             |
//   header code             compressed txout
//
//  - header code: 0xa8a258 (not coinbase, height 338156)
//  - compressed txout:
//    - 0x8ba5b9e763: VLQ-encoded compressed amount for 366875659 (3.66875659 BTC)
//    - 0x01: special script type pay-to-script-hash
//    - 0x1d...e6: script hash
// -----------------------------------------------------------------------------

// maxUint32VLQSerializeSize is the maximum number of bytes a max uint32 takes
// to serialize as a VLQ.

var maxUint32VLQSerializeSize = serializeSizeVLQ(1<<32 - 1)

//outpointkeypool defines a concurrent safe free list of byte slices used
//to provide tempory buffers for outpoint database keys.
var outpointKeyPool = sync.Pool{New: func() interface{} {
	b := make([]byte, chainhash.HashSize+maxUint32VLQSerializeSize)
	return &b
	//pointer to slice to avoid boxing alloc.
}}

//outpointkey returns a key suitable for use a database key in th utxo
//set while making use of a free list .a new buffer is allocated if
//there are not already any avaibable on the free list. the returned byte
// slice should be returned to the free list by using the reycleoutpointkey
//function when the caller id done with it_unless_ the slice will need to
//live for longer than the caller calculate such as when used to wirte to
//the database.
func outpointKey(outpoint wire.OutPoint) *[]byte {
	//a vlq employs an msg encoding .so they are useful not only to redue
	//the amount of storage space. but also so internation of utxos when
	//doing byte_wise comparisions will produce them in order.
	key := outpointKeyPool.Get().(*[]byte)
	idx := uint64(outpoint.Index)
	*key = (*key)[:chainhash.HashSize+serializeSizeVLQ(idx)]
	copy(*key, outpoint.Hash[:])
	putVLQ((*key)[chainhash.HashSize:], idx)
	return key
}

//recycleoutpointkey puts the provided bytes slice.which should have been
//obtained via the outpointKey function .back on the free list.
func recycleOutpointKey(key *[]byte) {
	outpointKeyPool.Put(key)
}

//utxoentryheadercode returns the calculted header code to be used when
//serializing the provided utxo entry.
func utxoEntryHeaderCode(entry *UtxoEntry) (uint64, error) {
	if entry.IsSpent() {
		return 0, AssertError("attmpt to serialize spent utxo header")

	}

	//as describe in the serialition format conments the header code
	//encodes the height shifted over on bit and the coinbase flag in
	//the lowest bit.
	headerCode := uint64(entry.BlockHeight()) << 1
	if entry.IsCoinBase() {
		headerCode |= 0x01
	}

	return headerCode, nil
}

//serializedutxo entry returns the entry serialized to a format that is
//is suitalbe for long-term storage .the format is describe in detail above
func serializeUtxoEntry(entry *UtxoEntry) ([]byte, error) {
	//spent outputs have no serialization
	if entry.IsSpent() {
		return nil, nil
	}

	//encode the header code.
	headerCode, err := utxoEntryHeaderCode(entry)
	if err != nil {
		return nil, err
	}

	//calculate the size needed to serialize the entry
	size := serializeSizeVLQ(headerCode) +
		compressedTxOutSize(uint64(entry.Amount()), entry.PkScript())

	//serialize the header code followed by the compressed unspent trnasaction
	//output.
	serialized := make([]byte, size)
	offset := putVLQ(serialized, headerCode)
	offset += putCompressedTxOut(serialized[offset:], uint64(entry.Amount()), entry.PkScript())

	return serialized, nil
}

//deserializeutxentry decodes a uxto entry from the passed serialized byte
//slice into a new uxtoentry using  a format that is suitable for long-term
//sotorage the format is descirible in detail above.
func deserializeUtxoEntry(serialized []byte) (*UtxoEntry, error) {
	//deserialize the header code.
	code, offset := deserializeVLQ(serialized)
	if offset >= len(serialized) {
		return nil, errDeserialize("unexpeted end of data after header")
	}

	//decode the header code.
	//bit 0 indicates whether the containing transaction is a coinbase .
	//bit 1-x encode height of containing trnasaction
	isCoinBase := code&0x01 != 0
	blockHeight := int32(code >> 1)

	//decode the compressed unspent transaction output.
	amount, pkScript, _, err := decodeCompressedTxOut(serialized[offset:])
	if err != nil {
		return nil, errDeserialize(fmt.Sprintf("unable to decode "+
			"utxo :%v", err))
	}

	entry := &UtxoEntry{
		amount:      int64(amount),
		pkScript:    pkScript,
		blockHeight: blockHeight,
		packedFlags: 0,
	}

	if isCoinBase {
		entry.packedFlags |= tfCoinBase
	}

	return entry, nil
}

//dbfetchutxoentrybyhash attempts to find and fetch a utxo for the given hash
//it uses a cursor and seek to try and do this as efficiently as possible.
//when there are no entries for the provided hash nil will be returned for the
//both the entry and the error.
func dbFetchUtxoEntryByHash(dbTx database.Tx, hash *chainhash.Hash) (*UtxoEntry, error) {
	//attempt to find an entry by seeking for the hash along with a zero
	//index ,dut to the fact the keys are serialized as <hash><index> ,
	//where the index uses an msb encoding .if there are any entries  for
	//the hash at all .one will be found.

	cursor := dbTx.Metadata().Bucket(utxoSetBucketName).Cursor()
	key := outpointKey(wire.OutPoint{Hash: *hash, Index: 0})
	ok := cursor.Seek(*key)
	recycleOutpointKey(key)
	if !ok {
		return nil, nil
	}

	//an entry was found .but it could just be an entry with the next
	//highest hash after the requested one .so make sure the hashes
	//acutally match .
	cursorKey := cursor.Key()
	if len(cursorKey) < chainhash.HashSize {
		return nil, nil
	}
	if !bytes.Equal(hash[:], cursorKey[:chainhash.HashSize]) {
		return nil, nil
	}

	return deserializeUtxoEntry(cursor.Value())

}

//dbfetchutxo entry uses an existing database trnasaction to fetch the specifed
//trnasaction output form the utxo set.
//when threr is no entry for the provided output .nil will be returned for both
//the entry and the error.
func dbFetchUtxoEntry(dbTx database.Tx, outpoint wire.OutPoint) (*UtxoEntry, error) {
	//fetch the unspent transaction output information for the passed
	//transaction output.return now when there is no netry .
	key := outpointKey(outpoint)
	utxoBucket := dbTx.Metadata().Bucket(utxoSetBucketName)
	serializeUtxo := utxoBucket.Get(*key)
	recycleOutpointKey(key)
	if serializeUtxo == nil {
		return nil, nil
	}

	//a non-nil zero-length entry meas there is an entry in the database
	//for a spent trnasation output which should never be the case.
	if len(serializeUtxo) == 0 {
		return nil, AssertError(fmt.Sprintf("database contains entry"+
			"for spent tx output %v ", outpoint))

	}

	//deserialize the utxo entry and return it.
	entry, err := deserializeUtxoEntry(serializeUtxo)
	if err != nil {
		//ensure and deserialzation errors are returned as database
		//corruption errors .
		if isDeserializeErr(err) {
			return nil, database.Error{
				ErrorCode:   database.ErrCorruption,
				Description: fmt.Sprintf("corrupt utxo entry"+"for %v :%v", outpoint, err),
				Err:         nil,
			}
		}

		return nil, err
	}

	return entry, nil
}

//dbpututxoview uses an existing database transaction to update the utxo
//set in the databse  on the provoded utxo view contents and state .
//in particular .only the entries that have been markded as modified are
//writen  to the database.
func dbPutUtxoView(dbTx database.Tx, view *UtxoViewpoint) error {
	utxoBucket := dbTx.Metadata().Bucket(utxoSetBucketName)
	for outpoint, entry := range view.entries {
		//no need to update the database if the entry was not modified
		if entry == nil || !entry.isModified() {
			continue
		}

		//remove the utxo entry if it is spent .
		if entry.IsSpent() {
			key := outpointKey(outpoint)
			err := utxoBucket.delete(*key)
			recycleOutpointKey(key)
			if err != nil {
				return err
			}

			continue
		}

		//serialize and store the utxo entry .
		serialized, err := serializeUtxoEntry(entry)
		if err != nil {
			return err
		}

		key := outpointKey(outpoint)
		err = utxoBucket.Put(*key, serialized)

		//note:the key is intentionly not recycled here since the
		//database interface contranct prohibits modifitions. it will
		//be garbage collected normally when the database is done with
		//it .
		if err != nil {
			return err
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// The block index consists of two buckets with an entry for every block in the
// main chain.  One bucket is for the hash to height mapping and the other is
// for the height to hash mapping.
//
// The serialized format for values in the hash to height bucket is:
//   <height>
//
//   Field      Type     Size
//   height     uint32   4 bytes
//
// The serialized format for values in the height to hash bucket is:
//   <hash>
//
//   Field      Type             Size
//   hash       chainhash.Hash   chainhash.HashSize
// -----------------------

//dbputblockindex uses an existing database transaction to update or
//add the block index entries for the hash to height and height to hash
//mapping for the provided values .
func dbPutBlockIndex(dbTx database.Tx, hash *chainhash.Hash, height int32) error {
	//serialize the height for use in the index entries.
	var serializeHeight [4]byte
	byteOrder.PutUint32(serializeHeight[:], uint32(height))

	//add the block hash to height mapping to the index
	meta := dbTx.Metadata()
	hashindex := meta.Bucket(hashIndexBucketName)
	if err := hashindex.Put(hash[:], serializeHeight[:]); err != nil {
		return err
	}

	//add the block height to hash mapping to the index
	heightIndex := meta.Bucket(heightIndexBucketName)
	return heightIndex.Put(serializedHeight[:], hash[:])
}

//deremoveblockindex uses an existing database transaction remvoe block
//index entries from the hash to height to hash mappings for the provided
//values
func dbRemoveBlockIndex(dbTx database.Tx, hash *chainhash.Hash, height int32) error {
	//remove the block hash to height mapping
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(hashIndexBucketName)
	if err := hashIndex.Delete(hash[:]); err != nil {
		return err
	}

	//remove the block height to hash mapping
	var serializedHeight [4]byte
	byteOrder.PutUint32(serializedHeight[:], uint32(height))

	heightIndex := meta.Bucket(heightIndexBucketName)
	return heightIndex.Delete(serializedHeight[:])

}

//dbfetchheigthbyhash useds an existing database trasaction to retrive the
//height for the provided hash from the index
func dbFetchHeightByHash(dbTx database.Tx, hash *chainhash.Hash) (int32, error) {
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(hashIndexBucketName)
	serializedHeight := hashIndex.Get(hash[:])
	if serializedHeight == nil {
		str := fmt.Sprintf("block %s is not in the main chain ", hash)
		return 0, errNotInMainChain(str)
	}

	return int32(byteOrder.Uint32(serializedHeight)), nil
}

//dbfentchhashbyheight uses an existing database trnasaction to retrive the
//hash for the provided height from the index.
func dbFetchHashByHeight(dbTx database.Tx, height int32) (*chainhash.Hash, error) {
	var serializedHeight [4]byte
	byteOrder.PutUint32(serializedHeight[:], uint32(height))

	meta := dbTx.Metadata()
	heightIndex := meta.Bucket(heightIndexBucketName)
	hashBytes := heightIndex.Get(serializedHeight[:])
	if hashBytes == nil {
		str := fmt.Sprintf("no block at height %d exists ", height)
		return nil, errNotInMainChain(str)
	}

	var hash chainhash.Hash
	copy(hash[:], hashBytes)
	return &hash, nil

}

//-------
//the best chain state consits of the best block hash and heigth ,the total
//number of trnasations up to and including those in the best block ,and
//the accummlated work sum up to and including the best block.

//the serialized format is :
//-------

//bestchainstate represent the data to be stored the database for the
//current best chain state.
type bestChainState struct {
	hash      chainhash.Hash
	height    uint32
	totalTxns uint64
	workSum   *big.Int
}

//serializebestchainstate returns the serialztion of the passed block
//best chain state. this is data to be stored in the chain state bucket
func serializeBestChainState(state bestChainState) []byte {
	//calculate the full size needed to serialize the chain state.
	workSumBytes := state.workSum.Bytes()
	workSumBytesLen := uint32(len(workSumBytes))
	serializedLen := chainhash.HashSize + 4 + 8 + 4 + workSumBytesLen

	//serialize the chain state .

	// Serialize the chain state.
	serializedData := make([]byte, serializedLen)
	copy(serializedData[0:chainhash.HashSize], state.hash[:])
	offset := uint32(chainhash.HashSize)
	byteOrder.PutUint32(serializedData[offset:], state.height)
	offset += 4
	byteOrder.PutUint64(serializedData[offset:], state.totalTxns)
	offset += 8
	byteOrder.PutUint32(serializedData[offset:], workSumBytesLen)
	offset += 4
	copy(serializedData[offset:], workSumBytes)
	return serializedData[:]

}

//deserializebestchainstate deserializes the passed serialized best chain
//state .this is data stored in the chain state bucket and is updated after
//every block is connected or disconnected form the main chain

func deserializeBestChainState(serializedData []byte) (bestChainState, error) {

	//ensur the serialized data has enough bytes to properly deserialize
	//the hash,height ,total transactions and work sum length.
	if len(serializedData) < chainhash.HashSize+16 {
		return bestChainState{}, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: "corrupt best chain state",
		}
	}

	state := bestChainState{}
	copy(state.hash[:], serializedData[0:chainhash.HashSize])
	offset := uint32(chainhash.HashSize)
	state.height = byteOrder.Uint32(serializedData[offset : offset+4])
	offset += 4
	state.totalTxns = byteOrder.Uint64(serializedData[offset : offset+8])
	offset += 8
	workSumBytesLen := byteOrder.Uint32(serializedData[offset : offset+4])

	offset += 4

	//ensure the serialized data has enough bytes to deserialize the
	//work sum.
	if uint32(len(serializedData[offset:])) < workSumBytesLen {
		return bestChainState{}, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: "corruption best chain state",
		}

	}

	workSumBytes := serializedData[offset : offset+workSumBytesLen]
	state.workSum = new(big.Int).SetBytes(workSumBytes)

	return state, nil

}

//dbputbeststate uses a existing database trnasaction to update the bet
//chain state with the given parameters
func dbPutBestState(dbTx database.Tx, snapshot *BestState, workSum *big.Int) error {
	//serialize the current best chain state.
	serializedData := serializeBestChainState(bestChainState{
		hash:      snapshot.Hash,
		height:    uint32(snapshot.Height),
		totalTxns: snapshot.TotalTxns,
		workSum:   workSum,
	})

	//store the current best chain state into the database
	return dbTx.Metadata().Put(chainStateKeyName, serializedData)
}

//createchainstate initlizes both the database and the chain state to the
//genesis block ,this includes creating the necessary bukcet and inserting
//the genesis block .so it must only be called on an uninitialized database.
func (b *BlockChain) createChainState() error {
	//create a new node from the genesis block and set is as the best node.
	genesisBlock := btcutil.NewBlock(b.chainParams.GenesisBlock)
	genesisBlock.SetHeight(0)
	header := &genesisBlock.MsgBlock().Header
	node := newBlockNode(header, nil)
	node.status = statusDataStored | statusValid
	b.bestChain.setTip(node)

	//add the new node to the index wihich is used for faster lookups
	b.index.addNode(node)

	//initialize the state related to the best block .since it is the
	//genesis block .use its timestmp for the median time.
	numTxns := uint64(len(genesisBlock.MsgBlock().Transactions))
	blockSize := uint64(genesisBlock.MsgBlock().Serialize())
	blockWeight := uint64(GetBlockWeight(genesisBlock))
	b.stateSnapshot = newBestState(node, blockSize, blockWeight, numTxns, numTxns, time.Unix(node.timestamp, 0))

	//create the intiial the database chain state inlcuing creating the
	//necessary index buckets and inserting the genesis block
	err := b.db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()

		//create the bucket that houses the block index data.
		_, err := meta.CreateBucket(blockIndexBucketName)
		if err != nil {
			return err
		}

		//create the bucket that hourse the chain block hash to height
		//index .
		_, err = meta.CreateBucket(hashIndexBucketName)
		if err != nil {
			return err
		}

		//create the bucket that hourse the chain block height to hash
		//index
		_, err = meta.CreateBucket(heightIndexBucketName)
		if err != nil {
			return err
		}

		// Create the bucket that houses the spend journal data and
		// store its version.
		_, err = meta.CreateBucket(spendJournalBucketName)
		if err != nil {
			return err
		}
		err = dbPutVersion(dbTx, utxoSetVersionKeyName,
			latestUtxoSetBucketVersion)
		if err != nil {
			return err
		}

		// Create the bucket that houses the utxo set and store its
		// version.  Note that the genesis block coinbase transaction is
		// intentionally not inserted here since it is not spendable by
		// consensus rules.
		_, err = meta.CreateBucket(utxoSetBucketName)
		if err != nil {
			return err
		}
		err = dbPutVersion(dbTx, spendJournalVersionKeyName,
			latestSpendJournalBucketVersion)
		if err != nil {
			return err
		}

		// Save the genesis block to the block index database.
		err = dbStoreBlockNode(dbTx, node)
		if err != nil {
			return err
		}

		// Add the genesis block hash to height and height to hash
		// mappings to the index.
		err = dbPutBlockIndex(dbTx, &node.hash, node.height)
		if err != nil {
			return err
		}

		// Store the current best chain state into the database.
		err = dbPutBestState(dbTx, b.stateSnapshot, node.workSum)
		if err != nil {
			return err
		}

		// Store the genesis block into the database.
		return dbStoreBlock(dbTx, genesisBlock)

	})
	return err

}

//initchainstate attempts to load and intialize the chain state from the
//database .when the db does not yet contain any chain state .both it and the
//chain state are initialized to the genesis block.
func (b *BlockChain) initChainState() error {
	//determine the state of the chain database .we may need to initialize
	//everything from scratch or upgarade certain buckets
	var initialized, hasBlockIndex bool
	err := b.db.View(func(dbTx database.Tx) error {
		initialized = dbTx.Metadata().Get(chainStateKeyName) != nil
		hasBlockIndex = dbTx.Metadata().Bucket(blockIndexBucketName) != nil
		return nil
	})
	if err != nil {
		return err
	}

	if !initialized {
		//at this point the database has not already been initialized .so
		//initialize both it and the chain state to the genesis block.
		return b.createChainState()
	}
	if !hasBlockIndex {
		err := migrateBlockIndex(b.db)
		if err != nil {
			return nil
		}
	}

	//attempt to load the chain state from the database
	err = b.db.View(func(dbTx database.Tx) error {
		//fetch the sotored chain state from the database metabase
		//when it does.t exist .it means the database has not been
		//initialized for use with chain yet .so break out now to
		//allow that happen under a wiriable database transaction
		serializedData := dbTx.Metadata().Get(chainStateKeyName)
		log.Tracef("serialized chain state :%x", serializedData)
		state, err := deserializeBestChainState(serializedData)
		if err != nil {
			return err
		}

		//load all of the headers form the database for the konwn best
		//chain and construct the block index accoudingly Since the
		// number of nodes are already known, perform a single alloc
		// for them versus a whole bunch of little ones to reduce
		// pressure on the GC.
		log.Infof("loading block index...")

		blockIndexBucket := dbTx.Metadata().Bucket(blockIndexBucketName)

		//determine how many blocks will be loaded into the index so wen
		//can allocate the right amount.
		var blockCount int32
		cursor := blockIndexBucket.Cursor()
		for ok := cursor.First(); ok; ol = cursor.Next() {
			blockCount++
		}

		blockNodes := make([]blockNode, blockCount)

		var i int32
		var lastNode *blockNode

		cursor = blockIndexBucket.Cursor()
		for ok := cursor.First(); ok; ok = cursor.Next() {
			header, status, err := deserializeBlockRow(cursor.Value())
			if err != nil {
				return err
			}
			//determine the parent block node.since we iterate block headers
			//in order of height ,if the blocks are mostly linear there is a
			//very good chance the previous header processed is the parent
			var parent *blockNode
			if lastNode == nil {
				blockHash := header.Blockhash()
				if !blockHash.isEqual(b.chainParams.GenesisHash) {
					return AssertError(fmt.Sprintf("initChainState: Expected "+
						"first entry in block index to be genesis block, "+
						"found %s", blockHash))
				}
			} else if header.prevBlock == lastNode.hash {
				// Since we iterate block headers in order of height, if the
				// blocks are mostly linear there is a very good chance the
				// previous header processed is the parent.
				parent = lastNode
			} else {
				parent = b.index.LookupNode(&header.PrevBlock)
				if parent == nil {
					return AssertError(fmt.Sprintf("initChainState: Could "+
						"not find parent for block %s", header.BlockHash()))
				}
			}

			//initialize the block node for the block,connect it
			//and add it the the block index.
			node := &blockNodes[i]
			initBlockNode(node, header, parent)
			node.status = status
			b.index.addNode(node)

			lastNode = node
			i++

		}

		//set the best chain view to the stored best state.
		tip := b.index.LookupNode(&state.hash)
		if tip == nil {
			return AssertError(fmt.Sprintf("initChainState: cannot find "+
				"chain tip %s in block index", state.hash))
		}
		b.bestChain.setTip(tip)

		//load the raw block bytes for the best block.
		blockBytes, err := dbTx.FetchBlock(&state.hash)
		if err != nil {
			return err
		}
		var block wire.MsgBlock
		err = block.Deserialize(bytes.NewReader(blockBytes))
		if err != nil {
			return err
		}

		//as a final consistency check,we will run throngh all the nodes
		//which are ancestors of the current chain tip .and mark them as
		//valid if they are not already marked as such ,this is a safe assumption
		//all the block before the current tip are valid by defination.
		for iterNode := tip; iterNode != nil; iterNode = iterNode.parent {
			// If this isn't already marked as valid in the index, then
			// we'll mark it as valid now to ensure consistency once
			// we're up and running.
			if !iterNode.status.KnownInvalid() {
				log.Infof("Block %v (height=%v) ancestor of "+
					"chain tip not marked as valid, "+
					"upgrading to valid for consistency",
					iterNode.hash, iterNode.height)
				b.index.SetStatusFlags(iterNode, statusValid)
			}
		}

		//initiaze the state related to the best block.
		blockSize := uint64(len(blockBytes))
		blockWeight := uint64(GetBlockWeight(btcutil.NewBlock(&block)))
		numTxns := uint64(len(block.Transactions))
		b.stateSnapshot = newBestState(tip, blockSize, blockWeight, numTxns, state.totalTxns, tip.CalcPastMedianTime())

		return nil

	})
	if err != nil {
		return err
	}

	// As we might have updated the index after it was loaded, we'll
	// attempt to flush the index to the DB. This will only result in a
	// write if the elements are dirty, so it'll usually be a noop.
	return b.index.flushToDB()

}

//deserializeblockrow parses a value in the block index bucket into a
//a block header and block status bitfield
func deserializeBlockRow(blockRow []byte) (*wire.BlockHeader, blockStatus, error) {
	buffer := bytes.NewReader(blockRow)

	var header wire.BlockHeader
	err := header.Deserialize(buffer)
	if err != nil {
		return nil, statusNone, err
	}

	statusByte, err := buffer.ReadByte()
	if err != nil {
		return nil, statusNone, err
	}

	return &header, blockStatus(statusByte), nil
}

//dbfetchheaderbyhash uses an existing database transaction to retrive the
//block header for the provided hash.
func dbFetchHeaderByHash(dbTx database.Tx, hash *chainhash.Hash) (*wire.BlockHeader, error) {
	headerBytes, err := dbTx.FetchBlockHeader(hash)
	if err != nil {
		return nil, err
	}

	var header wire.BlockHeader
	err = header.Deserialize(bytes.NewReader(headerBytes))
	if err != nil {
		return nil, err
	}

	return &header, nil
}

//dbfetchheaderbyheight uses an existing database transaction to retrive the
//block header for the provided height .
func dbFetchHeaderByHeight(dbTx database.Tx, height int32) (*wire.BlockHeader, error) {
	hash, err := dbFetchHashByHeight(dbTx, height)
	if err != nil {
		return nil, err
	}

	return dbFetchHeaderByHash(dbTx, hash)
}

//dbfetnchblockbynode uses an existing database trnasnaction to retrie
//the raw block for the provided node. deserialize it .and return  a btcutil.blcok
//with the height set.
func dbFetchBlockByNode(dbTx database.Tx, node *blockNode) (*btcutil.Block, error) {
	//load the raw block bytes form the database
	blockBytes, err := dbTx.FetchBlock(&node.hash)
	if err != nil {
		return nil, err
	}

	//create the encapsulated block and set the height appropriately.
	block, err := btcutil.NewBlockFromBytes(blockBytes)
	if err != nil {
		return nil, err
	}
	block.SetHeight(node.height)
	return block, nil
}

//dbstoreblocknode stores the block header and validation status to the block
//index bucket .this overwrite the current entry if there exists one.
func dbStoreBlockNode(dbTx database.Tx, node *blockNode) error {
	//serialize block data to be stored .
	w := bytes.NewBuffer(make([]byte, 0, blockHdrSize+1))
	header := node.Header()
	err := header.Serialize(w)
	if err != nil {
		return err
	}

	err = w.WriteByte(byte(node.status))
	if err != nil {
		return err
	}

	value := w.Bytes()

	//write block header data to block index bucket .
	blockIndexBucket := dbTx.Metadata().Bucket(blockIndexBucketName)
	key := blockIndexKey(&node.hash, uint32(node.height))
	return blockIndexBucket.Put(key, value)
}

// dbStoreBlock stores the provided block in the database if it is not already
// there. The full block data is written to ffldb.
func dbStoreBlock(dbTx database.Tx, block *btcutil.Block) error {
	hasBlock, err := dbTx.HasBlock(block.Hash())
	if err != nil {
		return err
	}
	if hasBlock {
		return nil
	}
	return dbTx.StoreBlock(block)
}

// blockIndexKey generates the binary key for an entry in the block index
// bucket. The key is composed of the block height encoded as a big-endian
// 32-bit unsigned int followed by the 32 byte block hash.
func blockIndexKey(blockHash *chainhash.Hash, blockHeight uint32) []byte {
	indexKey := make([]byte, chainhash.HashSize+4)
	binary.BigEndian.PutUint32(indexKey[0:4], blockHeight)
	copy(indexKey[4:chainhash.HashSize+4], blockHash[:])
	return indexKey
}

//blockbyheight returns the block at the given hegiht in the main chain
//this function is safe for concurrent access
func (b *BlockChain) BlockByHeight(blockHeight int32) (*btcutil.Block, error) {
	//lookup the block height in the best chain
	node := b.bestChain.NodeByHeight(blockHeight)
	if node == nil {
		str := fmt.Sprintf("no block at height %d exists", blockHeight)
		return nil, errNotInMainChain(str)
	}

	//load the block form the database and return it
	var block *btcutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block, err = dbFetchBlockByNode(dbTx, node)
		return err
	})

	return block, err

}
//blockbyhash returns the block form teh main chain withe given hash with
//the appropriate chain height set.
//this fucntion is safe for concurrent access.
func(b *BlockChain) BlockByHash(hash *chainhash.Hash)(*btcutil.Block,error) {
	//lookup the block hash in block index and ensure it is in the best
	//chain
	node := b.index.LookupNode(hash)
	if node == nil || !b.bestChain.Contains(node){
		str := fmt.Sprintf("block %s is not in the main chain",hash)
		return nil ,errNotInMainChain(str)

	}


	//load the block from the database and return it.
	var block *btcutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		block,err = dbFetchBlockByNode(dbTx,node)
		return err
	})
	return block ,err
}

//over












