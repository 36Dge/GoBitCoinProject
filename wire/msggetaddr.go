package wire

import "io"

//msggetaddr implements the message interface and represent a bitcoin getaddr
//message ,it its used to request a list of known active peers on the network
//from a peet to help indentify potential nodes.the list is returned via one
//or more addr messages

//this message has no payload
type MsgGetAddr struct {
}

//btcdecode decoder r using the bitcoin protocol encoding into the receiver
//this is part of the message interface implementation.
func (msg *MsgGetAddr) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	return nil
}

//btencode encodes the receiver to w using the bitcoin protocol encoding.
//this is part of the message interface implementation.
func (msg *MsgGetAddr) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	return nil
}

//command returns the protocol command string for the message .this is part
//of the message interface implementation.
func (msg *MsgGetAddr) Command() string {
	return CmdGetAddr
}

//newmsggetaddr returns a new bitcoin getaddr message that conforms to the message
//interface. see msggetaddr for details.
func NewMsgGetAddr() *MsgGetAddr {
	return &MsgGetAddr{}
}
//over













