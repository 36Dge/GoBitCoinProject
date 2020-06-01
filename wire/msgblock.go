package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"bytes"
	"fmt"
	"io"
)

// defaultTransactionAlloc is the default size used for the backing array
// for transactions.  The transaction array will dynamically grow as needed, but
// this figure is intended to provide enough space for the number of
// transactions in the vast majority of blocks without needing to grow the
// backing array multiple times.
const defaultTransactionAlloc = 2048

// MaxBlocksPerMsg is the maximum number of blocks allowed per message.
const MaxBlocksPerMsg = 500

// MaxBlockPayload is the maximum bytes a block message can be in bytes.
// After Segregated Witness, the max block payload has been raised to 4MB.
const MaxBlockPayload = 4000000

// maxTxPerBlock is the maximum number of transactions that could
// possibly fit into a block.
const maxTxPerBlock = (MaxBlockPayload / minTxPayload) + 1

//txloc holds locator data for the offset and lenght of where a transaction is
//located within a msgblock data buffer

type TxLoc struct {
	TxStart int
	TxLen   int
}

// MsgBlock implements the Message interface and represents a bitcoin
// block message.  It is used to deliver block and transaction information in
// response to a getdata message (MsgGetData) for a given block hash.
type MsgBlock struct {
	Header       BlockHeader
	Transactions []*MsgTx
}

//addtransaction adds a transaction to the message
func (msg *MsgBlock) AddTransaction(tx *MsgTx) error {
	msg.Transactions = append(msg.Transactions, tx)
	return nil
}

//cleartransactions removes all transactions from the message
func (msg *MsgBlock) ClearTransactions() {
	msg.Transactions = make([]*MsgTx, 0, defaultTransactionAlloc)
}

//btcdecode decodes r using the bitcoin protocol encoding into the
//receiver. this is a part of the message interface implement.
//see deserialize for decoding blocks stored to disk,such as in a
//database,as opposed to decoding blocks from the wire
func (msg *MsgBlock) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	err := readBlockHeader(r, pver, &msg.Header)
	if err != nil {
		return err
	}

	txCount, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	//prevent more transactions than could possibly fit into
	//a block.it would be possible to cause memeory exhaustion and
	//pancis without a sane upper bound on this count
	if txCount > maxTxPerBlock {
		str := fmt.Sprintf("too many transactions to fit into a block "+
			"[count %d ,max %d]", txCount, maxTxPerBlock)
		return messageError("MsgBlock.BtcDecode", str)
	}

	msg.Transactions = make([]*MsgTx, 0, txCount)
	for i := uint64(0); i < txCount; i++ {
		tx := MsgTx{}
		err := tx.BtcDecode(r, pver, enc)
		if err != nil {
			return err
		}
		msg.Transactions = append(msg.Transactions, &tx)
	}

	return nil
}

// Deserialize decodes a block from r into the receiver using a format that is
// suitable for long-term storage such as a database while respecting the
// Version field in the block.  This function differs from BtcDecode in that
// BtcDecode decodes from the bitcoin wire protocol as it was sent across the
// network.  The wire encoding can technically differ depending on the protocol
// version and doesn't even really need to match the format of a stored block at
// all.  As of the time this comment was written, the encoded block is the same
// in both instances, but there is a distinct difference and separating the two
// allows the API to be flexible enough to deal with changes.

func (msg *MsgBlock) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of BtcDecode.
	//
	// Passing an encoding type of WitnessEncoding to BtcEncode for the
	// MessageEncoding parameter indicates that the transactions within the
	// block are expected to be serialized according to the new
	// serialization structure defined in BIP0141.
	return msg.BtcDecode(r, 0, WitnessEncoding)
}

// DeserializeNoWitness decodes a block from r into the receiver similar to
// Deserialize, however DeserializeWitness strips all (if any) witness data
// from the transactions within the block before encoding them.
func (msg *MsgBlock) DeserializeNoWitness(r io.Reader) error {
	return msg.BtcDecode(r, 0, BaseEncoding)
}

//deserializeTxloc decodes r in the same manner deserialize does.but it takes
//a byte buffer instead of a generic reader and return a slice containing the
//start and length of each transaction within the raw data that is being deserialized.
func (msg *MsgBlock) DeserializeTxLoc(r *bytes.Buffer) ([]TxLoc, error) {

	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of existing wire protocol functions.
	err := readBlockHeader(r, 0, &msg.Header)
	if err != nil {
		return nil, err
	}
	txCount, err := ReadVarInt(r, 0)
	if err != nil {
		return nil, err
	}
	// Prevent more transactions than could possibly fit into a block.
	// It would be possible to cause memory exhaustion and panics without
	// a sane upper bound on this count.
	if txCount > maxTxPerBlock {
		str := fmt.Sprintf("too many transactions to fit into a block"+
			"[count%d ,max %d]", txCount, maxTxPerBlock)
		return nil, messageError("MsgBlock.DeserializeTXLoc", str)
	}

	//deserialize each transaction while keeping track of its location
	//within the byte stream
	msg.Transactions = make([]*MsgTx, 0, txCount)
	TxLocs := make([]TxLoc, txCount)
	for i := uint64(0); i < txCount; i++ {
		tx := MsgTx{}
		err := tx.Deserialize(r)
		if err != nil {
			return nil, err
		}

		msg.Transactions = append(msg.Transactions, &tx)
		TxLocs[i].TxLen = (fullLen - r.Len()) - TxLocs[i].TxStart

	}
	return TxLocs, nil

}

//btcencode encode the reciver to w using the bitcioin protocol encoding
//this is part of the message interface implementation. See serialize for
//encoding blocks to be stored to disk ,such as in a database,as opppsed to
//encoding blocks for the wire .
func (msg *MsgBlock) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	err := writeBlockHeader(w, pver, &msg.Header)
	if err != nil {
		return err
	}

	err = WriteVarInt(w, pver, uint64(len(msg.Transactions)))
	if err != nil {
		return err
	}

	for _, tx := range msg.Transactions {
		err = tx.BtcEncode(w, pver, enc)
		if err != nil {
			return err
		}
	}

	return nil

}

// Serialize encodes the block to w using a format that suitable for long-term
// storage such as a database while respecting the Version field in the block.
// This function differs from BtcEncode in that BtcEncode encodes the block to
// the bitcoin wire protocol in order to be sent across the network.  The wire
// encoding can technically differ depending on the protocol version and doesn't
// even really need to match the format of a stored block at all.  As of the
// time this comment was written, the encoded block is the same in both
// instances, but there is a distinct difference and separating the two allows
// the API to be flexible enough to deal with changes.

func (msg *MsgBlock) Serialize(w io.Writer) error {

	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of BtcEncode.
	//
	// Passing WitnessEncoding as the encoding type here indicates that
	// each of the transactions should be serialized using the witness
	// serialization structure defined in BIP0141.

	return msg.BtcEncode(w, 0, WitnessEncoding)
}

//serialzenowitness encodes a block to w using an identical format to serialize
// with all (if any) witness data stripped from all transactions.
//this method is provided in addition to the regular serialize,in order to
//allow onet to selectively encode transaction witness data to non-upgraded
//peers which are unaware of the new encoding
func (msg *MsgBlock) SerializeNoWitness(w io.Writer) error {
	return msg.BtcEncode(w, 0, BaseEncoding)
}

//serializesize returns the numbers of bytes it would take to serialize the block
//factoring in any witness data within transaction
func (msg *MsgBlock) SerializeSize() int {
	//block header bytes + serilaized varint size for the number of transactions
	n := BlockHeaderLen + VarIntSerializeSize(uint64(len(msg.Transactions)))

	for _, tx := range msg.Transactions {
		n += tx.SerializeSize()
	}
	return n
}

//serializesizestripped returns the number of bytes it would take to serialize
//the block ,excluding any witness data (if any)
func (msg *MsgBlock) SerializeSizeStripped() int {
	// Block header bytes + Serialized varint size for the number of
	// transactions.
	n := blockHeaderLen + VarIntSerializeSize(uint64(len(msg.Transactions)))
	for _, tx := range msg.Transactions {
		n += tx.SerializeSizeStripped()
	}
	return n
}

//command returns the protocol command string for the message .this is part
//of the message interface implement
func (msg *MsgBlock) Command() string {
	return CmdBlock
}

//maxpayloadlength returns the maximum length the payload can be for the receiver
//this is part of the message interface implementation.
func (msg *MsgBlock) MaxPayloadLength(pver uint32) uint32 {
	//block header at 80 bytes + transaction count + max transactions
	//which can vary up to maxblockpayload (including the block header
	//and transaction count)
	return MaxBlockPayload
}

//blockhash computes the block indentifier hash for this block
func (msg *MsgBlock) BlockHash() chainhash.Hash {
	return msg.Header.BlockHash()
}

//txhashes return a slice of hashes of all of transactions in this block

func (msg *MsgBlock) TxHashes() ([]chainhash.Hash, error) {

	hashList := make([]chainhash.Hash, 0, len(msg.Transactions))
	for _, tx := range msg.Transactions {
		hashList = append(hashList, tx.TxHash())
	}
	return hashList, nil
}

//nesmsgblock returns a new bitcoin block message that conforms to the
//message interface. see msgblock for details.

func NewMsgBlock(blockHeader *BlockHeader) *MsgBlock {
	return &MsgBlock{
		Header:       *blockHeader,
		Transactions: make([]*MsgTx,0,defaultTransactionAlloc),
	}
}

//over