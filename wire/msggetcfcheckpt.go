package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"io"
)

//msggetcfcheckpt is a request for filter headers at evenly spaced intervals
//throughout the blockchain history it allows to set the filtertype field to
//get headers in the chain of basic (0x00)or extended(0x01)headers.
type MsgGetCFCheckpt struct {
	FilterType FilterType
	StopHash   chainhash.Hash
}

//btcdecode decodes r using the bitcoin portocol encoding into the receiver.
//this is apart of the message interface implementation.
func (msg *MsgGetCFCheckpt) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	return readElement(r, &msg.StopHash)

}

//btcencode encodes the receiver to w using the bitcoin protocol encoding .
//this is part of the message interface implementation.
func (msg *MsgGetCFCheckpt) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	return writeElement(w, &msg.StopHash)
}

//command returns the protocol command string for the message .this is part of
//the message interface implementation.
func (msg *MsgGetCFCheckpt) Command() string {
	return CmdGetCFCheckpt
}

//maxpayloadlength returns the maximum length the payload can be for the receiver
//this is part of the message interface implementation.
func (msg *MsgGetCFCheckpt) MaxPayloadLength(pver uint32) uint32 {

	//filer type + uint32 + block hash.
	return 1 + chainhash.HashSize

}

//newmsgetcfchekcpt returns a new bitcoin getcfcheckpt message that  comforms to the
//message interface using the passed parameters and defaults for the remaining fields.

func NewMsgGetCFCheckpt(filterType FilterType, stopHash *chainhash.Hash) *MsgGetCFCheckpt {
	return &MsgGetCFCheckpt{
		FilterType: filterType,
		StopHash:   *stopHash,
	}
}
//over








