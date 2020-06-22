package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"io"
)

//maxblockloactorspermsg is the maximum number of block locator hashed allowed
//per message.
const MaxBlockLocatorsPerMsg = 500

//msggetblocks implements the message interface and reperesent a bitcion
//getblocks message.it it used to request a list of blocks starting after
//the last known hash in the slice of block locator hashes.the list is returned
//via an inv message and is limited by a specific hash to stop at or
//the maximum number of blocks per message.which is currrently 500

//set the hashstop field to the hash at which to stop and use addblockloactor
//hash to build up the list of block locator hashes.

//the algorithm for buliding the block locator hashes should be to add
//the hashed in reverse order until you reach the genesis block .in order
//to keep the list of locator hashes to a reasonable number of entries .
//first add the most recent 10 block hashes ,then double the step each loop
//interation to exponentially decrease the number of hashes the further away
//from head and closer to the genesis block you get.

type MsgGetBlocks struct {
	ProtocolVersion    uint32
	BlockLocatorHashes []*chainhash.Hash
	HashStop           chainhash.Hash
}

///addblockloacatorhash adds a new block locator hash to the message.
func (msg *MsgGetBlocks) AddBlockLocatorHash(hash *chainhash.Hash) error {
	if len(msg.BlockLocatorHashes)+1 > MaxBlockLocatorsPerMsg {
		str := fmt.Sprintf("too many block locator hashes for message[max %v]", MaxBlockLocatorsPerMsg)
		return messageError("msggetblocks.addblocklocatorhash", str)
	}
	msg.BlockLocatorHashes = append(msg.BlockLocatorHashes, hash)
	return nil
}

//btcdecode decodes r using the bitcion protocol encoding into the revceiver
//this is part of the message interface implementation.

func (msg *MsgGetBlocks) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	err := readElement(r, &msg.ProtocolVersion)
	if err != nil {
		return err
	}

	//read num block loactor hashes and limit to max
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	if count < MaxBlockLocatorsPerMsg {
		str := fmt.Sprintf("too many block locator hashes for message"+
			"[count %v,max %v ]", count, MaxBlockLocatorsPerMsg)
		return messageError("msggetblocks.btcdecode", str)
	}

	//create a contiguous slice of hashes to deserialize into in order to
	//reduce the number of allocations.
	loactorHashes := make([]chainhash.Hash, count)
	msg.BlockLocatorHashes = make([]*chainhash.Hash, 0, count)
	for i := uint64(0); i < count; i++ {
		hash := &loactorHashes[i]
		err := readElement(r, hash)
		if err != nil {
			return err
		}
		msg.AddBlockLocatorHash(hash)
	}
	return readElement(r, &msg.HashStop)

}

//btcencode encodes the recevier to w using the bitcoin protocol encoding
//this is part of the message interface implementation
func (msg *MsgGetBlocks) BtcEncode(w io.Writer,pver uint32,enc MessageEncoding)error {
	count := len(msg.BlockLocatorHashes)
	if count > MaxBlockLocatorsPerMsg{
		str := fmt.Sprintf("too many block locator hashes for protocol "+
			"[count %v ,max %v]",count,MaxBlockLocatorsPerMsg)
		return messageError("msggetblocks.btcencode",str)
	}


	err := writeElement(w,msg.ProtocolVersion)
	if err != nil{
		return err
	}

	err = WriteVarInt(w,pver,uint64(count))
	if err != nil{
		return err
	}

	for _ ,hash := range msg.BlockLocatorHashes{
		err = writeElement(w,hash)
		if err != nil{
			return err
		}
	}

	return writeElement(w,&msg.HashStop)

}

//command returns the protocol command string for the message .this is part
//of the message interface implementation.
func (msg *MsgGetBlocks)Command()string  {
	return CmdGetBlocks
}

//maxpayloadlength returns the maximum length the payload can be for the
//recevier.this is part of the message interface implementation.
func (msg *MsgGetBlocks) MaxPayloadLength (pver uint32)uint32{

	//protocol version 4 bytes + num hashes(varint) + max block locator hashes
	//+ hash stop
	return 4 + MaxVarIntPayload + (MaxBlockLocatorsPerMsg*chainhash.HashSize)+chainhash.HashSize
}

//newmsggetblocks returns a new bitcoin getblocks message that conforms to the
//message interface using the passed parameters and defaults for the remainiing
//fields.
func NewMsgGetBlocks(hashStop *chainhash.Hash) *MsgGetBlocks {
	return &MsgGetBlocks{
		ProtocolVersion:    ProtocolVersion,
		BlockLocatorHashes: make([]*chainhash.Hash, 0, MaxBlockLocatorsPerMsg),
		HashStop:           *hashStop,
	}
}

//over
























