package wire

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

//maxuseragentlen is the maximum allowed length for the agent field in a
//version message(msgversion)
const MaxUserAgentLen = 256

//defualt useragent for wire in the stack
const DefaultUserAgent = "/btcwire:0.5.0/"

//msgversion implements the message interface and represents a bitcoin version
//message it is used for a peer to advertise itselt as soon as an outbound
//connection is made.the remote peer then uses this information along with
//its own to negotiate.the remote peer must then respond with a version message
//of its own containing the negotinated values followed by a verack message .this
//exchange must take place before any further conmunication is allowed to preceed.
type MsgVersion struct {
	//version of the protocol the node is using
	ProtocolVersion int32

	//bitfield which indentifies the enable services
	Services ServiceFlag

	//time the message was generated. this is encoded as an int64  on the wire.
	Timestamp time.Time

	//address of the remote peer.
	AddrYou NetAddress

	//address of the local peer.
	AddrMe NetAddress

	//unique value associated with message that is used to detect self
	//connections.
	Nonce uint64

	//the uer agent that generated message .this is a encoded as a varstring
	//on the wire .this has a mx length of maxuseragentlen.
	UserAgent string

	//last block seen by the generator of the version message.
	LastBlock int32

	// Don't announce transactions to peer.
	DisableRelayTx bool
}

//hasservice returns whether the specified service is supported by the peer
//that generated the message.
func (msg *MsgVersion) HasService(service ServiceFlag) bool {
	return msg.Services&service == service
}

//addservices adds service as a supported service by the peer generated the messge
func (msg *MsgVersion) AddService(service ServiceFlag) {
	msg.Services |= service
}

//btcdecode decodes r using the bitcoin protocol encoding into hte receiver
//the version message is special in that the protocol version has not been
//negotiated yet.as a result .the pver field is ingonred and any fields which
//are added in new versions are optinonal .this also mean that r must be a
//*byte.buffer so the number of remaining bytes can be ascertained.
//this is part of the message interface implementation.
func (msg *MsgVersion) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("msgversion.btcdecode reader is not a " +
			"*bytes.buffer")
	}

	err := readElements(buf, &msg.ProtocolVersion, &msg.Services, (*int64Time)(&msg.Timestamp))
	if err != nil {
		return err
	}

	err = readNetAddress(buf, pver, &msg.AddrYou, false)
	if err != nil {
		return err
	}

	//protocol versions >= 106 andded a from address ,nonce,and user agent
	//field and they are only considered present if there are bytes remaining
	//in the message.

	if buf.Len() > 0 {
		err = readElement(buf, &msg.Nonce)
		if err != nil {
			return err
		}
	}

	if buf.Len() > 0 {
		userAgent, err := ReadVarString(buf, pver)
		if err != nil {
			return err
		}

		err = validateUserAgent(userAgent)
		if err != nil {
			return err
		}
		msg.UserAgent = userAgent
	}

	//protocol version >= 209 added a last known block field .it is only considered
	//present if there are bytes remaining in the message.
	if buf.Len() > 0 {
		err = readElement(buf, &msg.LastBlock)
		if err != nil {
			return err
		}
	}

	//there was no realy trasnaction field before bip0037version.but the default
	//behavior prior to the addition of the field was to always realy transaction .
	if buf.Len() > 0 {
		//it is safe to ignore the error here since the buffer has at least on
		//bytes and that byte will result in a boolean value regardless of its
		//value .also the wire encoding for the field is true when transactions
		//should be relayed. so reverse it for the disablerelayTx field.
		var relayTx bool
		readElement(r, &relayTx)
		msg.DisableRelayTx = !relayTx
	}

	return nil

}

//btcencode encodes the receiver to w using the bitcoin protocol encoding
//this is part of the message interface implementation.
func (msg *MsgVersion) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	err := validateUserAgent(msg.UserAgent)
	if err != nil {
		return err
	}

	err = writeElements(w, msg.ProtocolVersion, msg.Services, msg.Timestamp.Unix())
	if err != nil {
		return err
	}

	err = writeNetAddress(w, pver, &msg.AddrYou, false)
	if err != nil {
		return err
	}

	err = writeNetAddress(w, pver, &msg.AddrMe, false)
	if err != nil {
		return err
	}

	err = writeElement(w, msg.Nonce)
	if err != nil {
		return err
	}

	err = WriteVarString(w, pver, msg.UserAgent)
	if err != nil {
		return err
	}

	err = writeElement(w, msg.LastBlock)
	if err != nil {
		return err
	}

	//there was no relay trasnaction field before bip0037version.also.
	//the wire encoding for the field is true when transations should be
	//relay.so reverse it from the disablerelaytx field,
	if pver >= BIP0037Version {
		err = writeElement(w, !msg.DisableRelayTx)
		if err != nil {
			return err
		}

	}
	return nil
}

//command reutrns the protocol command string for the message .this is part
//of the message inteface implementation.
func (msg *MsgVersion) Command() string {
	return CmdVersion
}

//maxpayloadlength returns the maximum length the payload can be for the
//recevier this is part of the message interface implemation.
func (msg *MsgVersion) MaxPayloadLength(pver uint32) uint32 {

	// Protocol version 4 bytes + services 8 bytes + timestamp 8 bytes +
	// remote and local net addresses + nonce 8 bytes + length of user
	// agent (varInt) + max allowed useragent length + last block 4 bytes +
	// relay transactions flag 1 byte.
	return 33 + (maxNetAddressPayload(pver) * 2) + MaxVarIntPayload +
		MaxUserAgentLen

}

//newmsgvsion retruns a new bitcoin version message that conforms to the message
//interface using the passed parameters and defaults for the remaining fields.
func NewMsgVersion(me *NetAddress, you *NetAddress, nonce uint64, lastBlock int32) *MsgVersion {

	//limit the timestamp to one second precision since the protocol does not support better.
	return &MsgVersion{
		ProtocolVersion: int32(ProtocolVersion),
		Services:        0,
		Timestamp:       time.Unix(time.Now().Unix(), 0),
		AddrYou:         *you,
		AddrMe:          *me,
		Nonce:           nonce,
		UserAgent:       DefaultUserAgent,
		LastBlock:       lastBlock,
		DisableRelayTx:  false,
	}

}

//validateuseragent checks useragent length against maxuseragentlen
func validateUserAgent(userAgent string) error {
	if len(userAgent) > MaxUserAgentLen {
		str := fmt.Sprintf("user agent too long [len%v,max%]",
			len(userAgent), MaxUserAgentLen)
		return messageError("msgversion", str)
	}

	return nil

}

//adduseragent adds a user agent to the user agent string for the version
//message. the version string is not defined to any strict format.although
//it is recommanded to use the form "major,minor,reversion;
func (msg *MsgVersion) AddUserAgent(name string, version string, comments ...string) error {

	newUserAgent := fmt.Sprintf("%s:%s", name, version)
	if len(comments) != 0 {
		newUserAgent = fmt.Sprintf("%s(%s)", newUserAgent, strings.Join(comments, ";"))
	}
	newUserAgent = fmt.Sprintf("%s%s/", msg.UserAgent, newUserAgent)
	err := validateUserAgent(newUserAgent)
	if err != nil {
		return err
	}

	msg.UserAgent = newUserAgent
	return nil

}

//over

