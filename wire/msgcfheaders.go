package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"io"
)

const (

	//maxcfheaderpayload is the maximum byte size fo a commited filter header
	MaxCFHeaderPayload = chainhash.HashSize

	//maxcfheaderspermsg is the maximum number of commited filter headers
	//that can be in a single bitcion cfheaders message.
	MaxCFHeadersPerMsg = 2000
)

//msgcfheaders implemnets the message interface and represents a bitcoin
//cfheaders message. it it used deliver committed filter header information
//in response to a getchfeaders message (msggetcfheaders).the maximum header
//of committed filter headers per message in currently 2000,see msggetcfheaders
//for details on requesting the headers.

type MsgCFHeaders struct {
	FilterType       FilterType
	StopHash         chainhash.Hash
	PrevFilterHeader chainhash.Hash
	FiterHashes      []*chainhash.Hash
}

//addcfhash adds a new filter hash to the message
func (msg *MsgCFHeaders) AddCFHash(hash *chainhash.Hash) error {

	if len(msg.FiterHashes)+1 > MaxCFHeadersPerMsg {
		str := fmt.Sprintf("too many block headers in message[max%v]", MaxBlockHeadersPerMsg)
		return messageError("msgcfheaders.addcfhash", str)

	}

	msg.FiterHashes = append(msg.FiterHashes, hash)
	return nil
}

// btcdecode decodes r using the bitcoin protocin encoding into the reciver
//this is part of the message interface implemetation.
func (msg *MsgCFHeaders) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	//read filter type
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	//read stop hash
	err = readElement(r, &msg.StopHash)
	if err != nil {
		return err
	}

	//read prev filter header
	err = readElement(r, &msg.PrevFilterHeader)
	if err != nil {
		return err
	}

	//read number of filter headers
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	//limit to max committed filter headers per message
	if count > MaxCFHeadersPerMsg {
		str := fmt.Sprintf("too many commited filter headers for "+
			"message [count %v ,max %v]", count, MaxBlockHeadersPerMsg)
		return messageError("msgcfheaders.btcdecode", str)
	}

	//crete a contigous slice of hashes to deserilaize into in order to reduce
	//the number of allocations.
	msg.FiterHashes = make([]*chainhash.Hash, 0, count)
	for i := uint64(0); i < count; i++ {
		var cfh chainhash.Hash
		err := readElement(r, &cfh)
		if err != nil {
			return err
		}
		msg.AddCFHash(&cfh)

	}

	return nil

}

//btcencode eccodes the receiver to w using the bitcoin protocol encoding
//this is part of the message interface implementation.
func (msg *MsgCFHeaders) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {

	//write filter type
	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	//write stop hash
	err = writeElement(w, msg.StopHash)
	if err != nil {
		return err
	}

	//write prev filter header
	err = writeElement(w, msg.PrevFilterHeader)
	if err != nil {
		return err
	}

	//limit to max committed headers per message.
	count := len(msg.FiterHashes)
	if count > MaxCFHeadersPerMsg {
		str := fmt.Sprintf("too many committed filter headers for"+
			"message[count%v ,max %v]", count, MaxBlockHeadersPerMsg)
		return messageError("msgcheaders.btcencode", str)
	}

	err = WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, cfh := range msg.FiterHashes {
		err := writeElement(w, cfh)
		if err != nil {
			return err
		}
	}

	return nil

}

//deserialize decodes a filter header from r into the receiver using a format
//that is suitable for long-term storage such as a database.this function differs
//from btcdecodes in that btcdecode decodes from the bitcion wire protocol as
//it was sent across the network .the wire encodeing can technically differ
//depeing on the protocol version and does not even really need to macth the
//format the format of a stored filter haeder at all .as of the time this comment
//was written ,the encoded filter header is the same in both instance ,but
//there is a distinct differcnce and separating the two allows the api to be
//flexible enough to deal with change.
func (msg *MsgCFHeaders) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// and the stable long-term storage format.  As a result, make use of
	// BtcDecode.
	return msg.BtcDecode(r, 0, BaseEncoding)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgCFHeaders) Command() string {
	return CmdCFHeaders
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver.this is part of the message interface implementation.
func (msg *MsgCFHeaders) MaxPayloadLegth(pver uint32) uint32 {

	//hash size + filter type + numheaders(varint)+
	//(header size * max headers)
	return 1 +chainhash.HashSize + chainhash.HashSize + MaxVarIntPayload+
		(MaxCFHeaderPayload * MaxCFHeadersPerMsg)

}

// NewMsgCFHeaders returns a new bitcoin cfheaders message that conforms to
// the Message interface. See MsgCFHeaders for details.
func NewMsgCFHeaders() *MsgCFHeaders {
	return &MsgCFHeaders{
		FilterHashes: make([]*chainhash.Hash, 0, MaxCFHeadersPerMsg),
	}
}

//over









