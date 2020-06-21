package wire

import (
	"fmt"
	"io"
)

//msgfilterclear implements the message interface and represents a
//bitcion filterclear message which is used to reset a bloom filter.

//this message was not added until protocol version bip0037version and
//has no payload
type MsgFilterClear struct{}

//btcdecode decodes r using the bitcoin portocol encoding into the
//receiver. this is part of the message interface implementation.

func (msg *MsgFilterClear) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	if pver < BIP0037Version {
		str := fmt.Sprintf("filterclear message invalid for protocol"+
			"version %d", pver)
		return messageError("msgfilterclear.btcdecode", str)
	}

	return nil

}

//btcencode encodes the receiver to w using the biccoin protocol encding
//this is part of the message interface implementation.
func (msg *MsgFilterClear) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {

	if pver < BIP0037Version {
		str := fmt.Sprintf("filterclear message invalid for protocol "+
			"version %d", pver)
		return messageError("msgfilterclear.btcencode", str)
	}
	return nil
}

//command returns the protocol command string for the message .this is part
//of the message interface implementation
func (msg *MsgFilterClear) Command() string {
	return CmdFilterClear
}

//maxpayloadlength returns the maximum length the payload can be
//for the receiver .this is part of the message interface implementation.
func (msg *MsgFilterClear) MaxPayloadLength(pver uint32) uint32 {
	return 0
}

//newmsgfilterclear returns a new bitcoin filterclear message that conforms
//to the message interface see msgFilterclear for details.
func NewMsgFilterClear() *MsgFilterClear {
	return &MsgFilterClear{}
}

//over





