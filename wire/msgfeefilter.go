package wire

import (
	"fmt"
	"io"
)

//msgfeefilter implements the message interface and represents a bitcoin
//feefilter message .it is used to request the receiving peer does not
//announce any transactions below the specifed minimum fee rate.

//this message was not added until protocol versions starting with feefilterversion.
type MsgFeeFilter struct {
	MinFee int64
}

//btcdecode decodes r using the bitcion porotocol encoding into the reciver.
//this is part of the message interface implementation.
func (msg *MsgFeeFilter) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	if pver < FeeFilterVersion {
		str := fmt.Sprintf("feefilter message invalid for protocol"+
			"version %d", pver)
		return messageError("msgFeefilter.btcdecode", str)
	}
	return readElement(r, &msg.MinFee)
}

//btcencode encodes the receiver to w using the bitcion protocol encoding
//this is part of the message interface implemenation.
func (msg *MsgFeeFilter) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {

	if pver < FeeFilterVersion {
		str := fmt.Sprintf("feefilter message invalid for protocol "+
			"version %d", pver)
		return messageError("messagefiler.bctencode", str)
	}
	return writeElement(w, msg.MinFee)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgFeeFilter) Command() string {
	return CmdFeeFilter
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver,this is part of teh message interface implementation.
func (msg *MsgFeeFilter) MaxPayloadLength(pver uint32) uint32 {
	return 8
}

//newmsgfeefilter returns a new feefilter message that conforms to the
//message interface .see msgfeefilter for details.
func NewMsgFeeFilter(minfee int64) *MsgFeeFilter {
	return &MsgFeeFilter{MinFee: minfee}
}

//over