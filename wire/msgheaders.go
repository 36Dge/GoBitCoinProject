package wire

import (
	"fmt"
	"io"
)

// MaxBlockHeadersPerMsg is the maximum number of block headers that can be in
// a single bitcoin headers message.
const MaxBlockHeadersPerMsg = 2000

//msgheaders implements the message interface and reperesents a bitcoin headers
//message,it it used to deliver block header information in response to a getheaders
//message.the maximum number of block headers per message is currently 2000
//see msggetheaders for details on requesting the headers
type MsgHeaders struct {
	Headers []*BlockHeader
}

//addblockheaders adds a new block headers to the message.
func (msg *MsgHeaders) AddBlockHeader(bh *BlockHeader) error {
	if len(msg.Headers)+1 > MaxBlockHeadersPerMsg {
		str := fmt.Sprintf("too many block headers in message [max%v]",
			MaxBlockHeadersPerMsg)
		return messageError("msgheaders.addblcokheader", str)
	}

	msg.Headers = append(msg.Headers, bh)
	return nil
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver .
//this is part of the message interface implementation.
func (msg *MsgHeaders) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	//limit to max block headers per message
	if count > MaxBlockHeadersPerMsg {
		str := fmt.Sprintf("too many block for message "+
			"[couont %v ,max %v]", count, MaxBlockHeadersPerMsg)
		return messageError("msgheaders.btcdecode", str)
	}

	//create a contigous slice of headers to deserialize into in or to
	//reduce the number of allocations.
	haders := make([]BlockHeader, count)
	msg.Headers = make([]*BlockHeader, 0, count)
	for i := uint64(0); i < count; i++ {
		bh := &haders[i]
		err := readBlockHeader(r, pver, bh)
		if err != nil {
			return err
		}

		txCount, err := ReadVarInt(r, pver)
		if err != nil {
			return err
		}

		//ensure the transaction count is zero for headers
		if txCount > 0 {
			str := fmt.Sprintf("block headers may not contain "+
				"transaction [count%v]", txCount)
			return messageError("msgHeaders.btcdecode", str)
		}

		msg.AddBlockHeader(bh)

	}

	return nil

}

//btcencode encode the receiver to w using the bitcoin protocol encoding .
//this is part of the message interface implementation.
func (msg *MsgHeaders) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {

	//limit to max block haders peer message
	count := len(msg.Headers)
	if count > MaxBlockHeadersPerMsg {
		str := fmt.Sprintf("too many block haeaders for message"+
			"[count %v,max %v]", count, MaxBlockHeadersPerMsg)
		return messageError("msgheaders.btcencode", str)
	}

	err := WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, bh := range msg.Headers {
		err := writeBlockHeader(w, pver, bh)
		if err != nil {
			return err
		}

		//the wire protocol encoding always inculude a 0 for the number
		//of transaction on header messages. this is really just an
		//artifact of the way the original implementation serializes
		//block haders ,but it is required.
		err = WriteVarInt(w, pver, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgHeaders) Command() string {
	return CmdHeaders
}


//maxpayloadlength returns the maximum length the payload can be for
//the receiver .this is part of the message interface implementation.
func (msg *MsgHeaders) MaxPayloadLength(pver uint32)uint32 {

	//num headers(varint) + max allowed hades(headers lenghth
	//+1 byte for the number of transactions which is always 0)
	return MaxVarIntPayload + ((MaxBlockHeadersPerMsg)+1 ) * MaxBlockHeadersPerMsg
}


//newmsgheaders returns a new bitcoin haders message that conform to the message interface
//see msgheaders for details.
func NewMsgHeaders() *MsgHeaders {
	return &MsgHeaders{Headers: make([]*BlockHeader,0,MaxBlockHeadersPerMsg)}
}

//over















