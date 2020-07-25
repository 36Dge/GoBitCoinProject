package wire

import (
	"fmt"
	"io"
)

//use the addinvvect function to build up the list of inventory vectory
//when sending a getdata message to another peer.
type MsgGetData struct {
	InvList []*InvVect
}

//addinvvect adds an inventory vector to the message
func (msg *MsgGetData) AddInvVect(iv *InvVect) error {
	if len(msg.InvList)+1 > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message[max %v] ", MaxInvPerMsg)
		return messageError("msggetdata.addinvvect", str)
	}

	msg.InvList = append(msg.InvList, iv)
	return nil
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver .
//this is part of the message interface implementation.
func (msg *MsgGetData) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	//limit to max inventory per message
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("msggetdata.btcdecode", str)
	}

	//create a contiuous slice of inventory vectors to deserrialize into in
	//order to reduce the number of allcocations.
	invList := make([]InvVect, count)
	msg.InvList = make([]*InvVect, 0, count)
	for i := uint64(0); i < count; i++ {
		iv := &invList[i]
		err := readInvVect(r, pver, iv)
		if err != nil {
			return err
		}

		msg.AddInvVect(iv)
	}
	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetData) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	// Limit to max inventory vectors per message.
	count := len(msg.InvList)
	if count > MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("MsgGetData.BtcEncode", str)
	}

	err := WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, iv := range msg.InvList {
		err := writeInvVect(w, pver, iv)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetData) Command() string {
	return CmdGetData
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetData) MaxPayloadLength(pver uint32) uint32 {
	//num invvetory vectory(varint) + max allowed inventory vectors
	return MaxVarIntPayload + (MaxInvPerMsg * maxInvVectPayload)

}

//newmsggetdata returns a new bitcoin getdata message that conforms to the 
//message interface see msggetdata for details.
func NewMsgGetData() *MsgGetData {
	return &MsgGetData{
		InvList: make([]*InvVect, 0, defaultInvListAlloc),
	}
}


func NewMsgGetDataSizeHint(sizeHint uint) *MsgGetData {
	// Limit the specified hint to the maximum allow per message.
	if sizeHint > MaxInvPerMsg {
		sizeHint = MaxInvPerMsg
	}

	return &MsgGetData{
		InvList: make([]*InvVect, 0, sizeHint),
	}
}


