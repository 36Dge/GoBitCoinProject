package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"io"
)

//filtertype is used to represent a filter type.
type FilterType uint8

const (

	// GCSFilterRegular is the regular filter type.
	GCSFilterRegular FilterType = iota
)

const (
	//maxcfilterdatasize is the maximum byte size of a commited filter.
	//the maximum size is currently defined as 256kib.
	MaxCFilterDataSize = 256 * 1024
)

//msgcfilter implements the message interface and represents a bitcoin cfilter
//message it is used to deliver a commited filter in response to a getcfilter
//message.
type MsgCFilter struct {
	FilterType FilterType
	BlockHash  chainhash.Hash
	Data       []byte
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver
//this is a part of the message interface implementation
func (msg *MsgCFilter) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {

	//read filter type
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	//read the hash of the filter,s block
	err = readElement(r, &msg.BlockHash)
	if err != nil {
		return err
	}

	//read filter data
	msg.Data, err = ReadVarBytes(r, pver, MaxCFilterDataSize, "cfilter data")
	return err
}

//btcencode encodes the recevier to w using the bitcoin protocol encoding
//this is part of message interface implementation.
func (msg *MsgCFilter) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	size := len(msg.Data)
	if size > MaxCFilterDataSize {
		str := fmt.Sprintf("cftiler size too large for message"+
			"[size %v ,max %v]", size, MaxCFilterDataSize)
		return messageError("msgcfitter.btcencode", str)
	}
	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	err = writeElement(w, msg.BlockHash)
	if err != nil {
		return err
	}

	return WriteVarBytes(w, pver, msg.Data)

}

// Deserialize decodes a filter from r into the receiver using a format that is
// suitable for long-term storage such as a database. This function differs
// from BtcDecode in that BtcDecode decodes from the bitcoin wire protocol as
// it was sent across the network.  The wire encoding can technically differ
// depending on the protocol version and doesn't even really need to match the
// format of a stored filter at all. As of the time this comment was written,
// the encoded filter is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgCFilter) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// and the stable long-term storage format.  As a result, make use of
	// BtcDecode.
	return msg.BtcDecode(r, 0, BaseEncoding)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgCFilter) Command() string {
	return CmdCFilter
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver ,this is part of message interface implementation.
func (msg *MsgCFilter) MaxPayloadLength(pver uint32) uint32 {
	return uint32(VarIntSerializeSize(MaxCFilterDataSize)) +
		MaxCFilterDataSize + chainhash.HashSize + 1

}

//newmsgcftiler returns a new bitcoin cfilter message that conforms to the
//message interface ,see msgcfiter for details
func NewMsgCFiler(filterType FilterType, blockHash *chainhash.Hash, data []byte) *MsgCFilter {
	return &MsgCFilter{
		FilterType: filterType,
		BlockHash:  *blockHash,
		Data:       data,
	}
}

//over