package wire

import "io"

//msgping implements the message interface and reprenents a bitcoin ping
//message.
//for versions bip0031version and earlier.it its used primarily to confirm
//that a connection is still valid .a transmission error is typically interpreted
//as a closed connection and that the peer should be removed .for version
//after bip 0031version it contains and identifier which can be returned in the pong
//message to determine network timing.
//the parload for this message just consists of a nonce used for identifying it
//later.
type MsgPing struct {
	//unique value associated with message that is used to identify
	//specific ping message.
	Nonce uint64
}

//btcdecode decodes r using the bitcion protocol encding into the receiver
//this is part of the message interface implementation
func (msg *MsgPing) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	//there was no nonce for bip0031version and erlier .
	//note:> is not a mistake here.the bip0031 was defined as after
	//the version unlike most others.
	if pver > BIP0031Version {
		err := readElement(r, &msg.Nonce)
		if err != nil {
			return err
		}
	}

	return nil

}

//btcencode encodes the recever to w using the bitcoin protocol encoding
//this is part the message interface implementation.
func (msg *MsgPing) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {

	if pver > BIP0031Version {
		err := writeElement(w, msg.Nonce)
		if err != nil {
			return err
		}
	}
	return nil

}

//commnad returns the protocol command string for the message .this is part
//of the message interface implementation.
func (msg *MsgPing) Command() string {
	return CmdPing
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver .this is part of the message interface implementation.
func (msg *MsgPing) MaxPayloadLength(pver uint32) uint32 {

	plen := uint32(0)

	if pver > BIP0031Version {
		//nonce 8 bytes
		plen += 8

	}

	return plen

}

//nesmsgping returns a new bitcoin ping message that conforms to the message
//interface see msgping for details.
func NewMsgPing(nonce uint64) *MsgPing {
	return &MsgPing{Nonce: nonce,}
}

//over
