package wire

import (
	"fmt"
	"io"
)

//amxaddrpermsg is the maximum number of addressed that can be
//in a bitcoin addr message
const MaxAddrPerMsg = 1000

//msgaddr inplenments the message interface and reperesents a bitcoin addr
//message,it is used to provide a list of known active peers on the network.
//an active peer is considered one that has transmitted a message within the
//last 3 hours,nodes which have not transmitted in that time frame should
//be forgotten,each message is limited to a maximum number of address,which
//is currently 1000,as a result,mutiple message must be used to relay the full
//list

//use the addaddress function to bulid up the list of known address when sending
//an message to anther peer.
type MsgAddr struct {
	AddrList []*NetAddress
}

//addaddress adds a known active peer to the message
func (msg *MsgAddr) AddAddress(na *NetAddress) error {
	if len(msg.AddrList)+1 > MaxAddrPerMsg {
		str := fmt.Sprintf("too many address in message [max%v]", MaxAddrPerMsg)
		return messageError("MsgAddr.AddAddress", str)
	}
	msg.AddrList = append(msg.AddrList, na)
	return nil
}

//addadress adds multiple known active peers to the message
func (msg *MsgAddr) AddAddresses(netAddrs ...*NetAddress) error {
	for _, na := range netAddrs {
		err := msg.AddAddress(na)
		if err != nil {
			return err
		}
	}
	return nil
}

//clearaddresses removes all addresseds from the message.
func (msg *MsgAddr) ClearAddressed() {
	msg.AddrList = []*NetAddress{}
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver
//this is part of the Message interface implementation
func (msg *MsgAddr) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// limit to max addresses per message.
	if count > MaxAddrPerMsg {
		str := fmt.Sprintf(" too many address for message "+"[count%v,max %v]", count, MaxInvPerMsg)
		return messageError("MsgAddr.BtcDecode", str)

	}

	addList := make([]NetAddress, count)
	msg.AddrList = make([]*NetAddress, 0, count)
	for i := uint64(0); i < count; i++ {
		na := &addList[i]
		err := readNetAddress(r, pver, na, true)
		if err != nil {
			return err
		}
		msg.AddAddress(na)
	}
	return nil
}

//btcencode encodes the receiver to w using the bitcoin protocol encoding .
//this is part of the message interface implementation
func (msg *MsgAddr) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	//protocol versions before multipleaddressversion only allow i address
	//per message
	count := len(msg.AddrList)
	if pver < MultipleAddressVersion && count > 1 {
		str := fmt.Sprintf("too many address for message of "+"protocol version %v [count %v max 1]", pver, count)
		return messageError("MsgAddr.BtcEncode", str)
	}

	if count > MaxAddrPerMsg {

		str := fmt.Sprintf("too many addresses for message "+
			"[count %v, max %v]", count, MaxAddrPerMsg)
		return messageError("MsgAddr.BtcEncode", str)
	}

	err := WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, na := range msg.AddrList {
		err = writeNetAddress(w, pver, na, true)
		if err != nil {
			return err
		}
	}

	return nil

}

//command returns the protocol command string for the message .this is part of
// the message interface implementation
func (msg *MsgAddr) Command() string {
	return CmdAddr
}

//maxpayloadlength returns the maximum length the payload can be for the receiver
//this is part of the message interface implementation .
func (msg *MsgAddr) MaxPayloadLength(pver uint32) uint32 {
	//num address(varint)+a single net address.
	return MaxVarIntPayload + maxNetAddressPayload(pver)

	// Num addresses (varInt) + max allowed addresses.
	return MaxVarIntPayload + (MaxAddrPerMsg * maxNetAddressPayload(pver))
}

//newmsgaddr returns a new bitcion addr message that conforms to the message interface
//see msgaddr for details.
func NewMsgAddr() *MsgAddr {
	return &MsgAddr{AddrList: make([]*NetAddress, 0, MaxAddrPerMsg)}
}

//over
