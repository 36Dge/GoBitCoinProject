package wire

import (
	"fmt"
	"io"
)

const (
	//maxfilteradddatasize is the maximum byte size of a data
	//element to add to the bloom filter.it is equal to the
	//maximum element size of a script.
	MaxFilterAddDataSize = 530
)

//msgfilteradd implements the message interface and represents a bitcion filteradd
//message.it is used to add a data element to an existing bloom filter

//this message was not added until protocol version bip0037version.
type MsgFilterAdd struct {
	Data []byte
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver.
//this is part of the message interface implementation.
func (msg *MsgFilterAdd) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	if pver < BIP0037Version {
		str := fmt.Sprintf("filteradd message invalid for protocol"+
			"version%d", pver)
		return messageError("msgfilteradd.btcdecode", str)
	}

	var err error
	msg.Data, err = ReadVarBytes(r, pver, MaxFilterAddDataSize, "filteradd data")
	return err
}

//btcencode encodes the recevier to w using the bitcion protocol encoding
//this is part of the message interface implementation.
func (msg *MsgFilterAdd) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {

	if pver < BIP0037Version{
		str := fmt.Sprintf("filter message invalid for protocol "+
			"version %d",pver)
		return messageError("msgfilteradd.btcencode",str)
	}

	size := len(msg.Data)
	if size > MaxFilterAddDataSize{
		str := fmt.Sprintf("filteradd size too large for message"+
			"[size%v,max %v]",size,MaxFilterAddDataSize)
		return messageError("msgfilteradd.btcencode",str)
	}


	return WriteVarBytes(w,pver,msg.Data)

}

//command returns the protocol command string for the message .this is part
//of the message interface implementation.
func (msg *MsgFilterAdd)Command()string	 {
	return CmdFilterAdd
}

//maxpayloadlenght returns the maximum length the paylaod can be for the
//receiver.this is part of message interface implementation.
func(msg *MsgFilterAdd)MaxPayloadLength(pver uint32)uint32{
	return uint32(VarIntSerializeSize(MaxFilterAddDataSize))+
		MaxFilterAddDataSize
}

//newmsgfilteradd returns a new bitcion filteradd message that conforms to the
//message interface see msgfilteradd for details.
func NewMsgFilterAdd(data []byte) *MsgFilterAdd {
	return &MsgFilterAdd{Data: data}
}

//over















