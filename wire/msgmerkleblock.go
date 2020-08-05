package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"io"
)

//maxflagspermerkleblock is the maximum number of flag bytes that cound
//possibly fit into a merkle block. since each trnasaction is represented
//by a single bit. this is the max number of transactions per block divided
//by 8 bits per byte .then an extra one to conver partials.
const maxFlagsPerMerkleBlock = maxTxPerBlock / 8

//msgmerkleblock implements the message interface and reperents a bitcoin
//merkleblock message which is used to reset a bloom filter.

//this message was not added until protocol version bip0037version.
type MsgMerkleBlock struct {
	Header      BlockHeader
	Transactions uint32
	Hashes      []*chainhash.Hash
	Flags       []byte
}

//addtxhash adds a new transaction hash to the message.
func (msg *MsgMerkleBlock) AddTxHash(hash *chainhash.Hash) error {
	if len(msg.Hashes)+1 > maxTxPerBlock {
		str := fmt.Sprintf("too many tx hashes for message [max %v]", maxTxPerBlock)
		return messageError("msgmerkleblock.addtxhash", str)
	}

	msg.Hashes = append(msg.Hashes, hash)
	return nil
}

//btcdecode decodes r using the bitcion protocol encoding into the receiver
//this is part of the message interface implementatin.
func (msg *MsgMerkleBlock) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < BIP0037Version {
		str := fmt.Sprintf("merkleblock message invalid for protocol"+
			"version %d", pver)
		return messageError("msgmerkleblock.btcdecode", str)
	}

	err := readBlockHeader(r, pver, &msg.Header)
	if err != nil {
		return err
	}

	err = readElement(r, &msg.Transactions)
	if err != nil {
		return err
	}

	//read num block locator hashes and limit to max
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if count > maxTxPerBlock {
		str := fmt.Sprintf("too many transaction hashes for message "+
			"[count %v,max %v]", count, maxTxPerBlock)
		return messageError("msgmerkleblock.btcdecode", str)
	}

	//create a contigous slice of hashes to deserialize into in order to
	//reduce the number of allocation.
	hashes := make([]chainhash.Hash, count)
	msg.Hashes = make([]*chainhash.Hash, 0, count)
	for i := uint64(0); i < count; i++ {
		hash := &hashes[i]
		err := readElement(r, hash)
		if err != nil {
			return err
		}
		msg.AddTxHash(hash)
	}

	msg.Flags, err = ReadVarBytes(r, pver, maxFlagsPerMerkleBlock, "merkle block flags size")

	return err

}


// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgMerkleBlock) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < BIP0037Version {
		str := fmt.Sprintf("merkleblock message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgMerkleBlock.BtcEncode", str)
	}

	// Read num transaction hashes and limit to max.
	numHashes := len(msg.Hashes)
	if numHashes > maxTxPerBlock {
		str := fmt.Sprintf("too many transaction hashes for message "+
			"[count %v, max %v]", numHashes, maxTxPerBlock)
		return messageError("MsgMerkleBlock.BtcDecode", str)
	}
	numFlagBytes := len(msg.Flags)
	if numFlagBytes > maxFlagsPerMerkleBlock {
		str := fmt.Sprintf("too many flag bytes for message [count %v, "+
			"max %v]", numFlagBytes, maxFlagsPerMerkleBlock)
		return messageError("MsgMerkleBlock.BtcDecode", str)
	}

	err := writeBlockHeader(w, pver, &msg.Header)
	if err != nil {
		return err
	}

	err = writeElement(w, msg.Transactions)
	if err != nil {
		return err
	}

	err = WriteVarInt(w, pver, uint64(numHashes))
	if err != nil {
		return err
	}
	for _, hash := range msg.Hashes {
		err = writeElement(w, hash)
		if err != nil {
			return err
		}
	}

	return WriteVarBytes(w, pver, msg.Flags)
}

//command returns the protocol command string for the message .this is patr
//of the message interface implementation.
func (msg *MsgMerkleBlock)Command()string  {
	return CmdMerkleBlock
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver. this is part of the message interface implementation.
func (msg *MsgMerkleBlock)MaxPayloadLength(pver uint32)uint32  {
	return  MaxBlockPayload
}

// NewMsgMerkleBlock returns a new bitcoin merkleblock message that conforms to
// the Message interface.  See MsgMerkleBlock for details.
func NewMsgMerkleBlock(bh *BlockHeader) *MsgMerkleBlock {
	return &MsgMerkleBlock{
		Header:       *bh,
		Transactions: 0,
		Hashes:       make([]*chainhash.Hash, 0),
		Flags:        make([]byte, 0),
	}
}
//over




















