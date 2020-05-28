package wire

import (
	"bytes"
	"fmt"
	"io"
)

// MsgAlert contains a payload and a signature:
//
//        ===============================================
//        |   Field         |   Data Type   |   Size    |
//        ===============================================
//        |   payload       |   []uchar     |   ?       |
//        -----------------------------------------------
//        |   signature     |   []uchar     |   ?       |
//        -----------------------------------------------
//
// Here payload is an Alert serialized into a byte array to ensure that
// versions using incompatible alert formats can still relay
// alerts among one another.
//
// An Alert is the payload deserialized as follows:
//
//        ===============================================
//        |   Field         |   Data Type   |   Size    |
//        ===============================================
//        |   Version       |   int32       |   4       |
//        -----------------------------------------------
//        |   RelayUntil    |   int64       |   8       |
//        -----------------------------------------------
//        |   Expiration    |   int64       |   8       |
//        -----------------------------------------------
//        |   ID            |   int32       |   4       |
//        -----------------------------------------------
//        |   Cancel        |   int32       |   4       |
//        -----------------------------------------------
//        |   SetCancel     |   set<int32>  |   ?       |
//        -----------------------------------------------
//        |   MinVer        |   int32       |   4       |
//        -----------------------------------------------
//        |   MaxVer        |   int32       |   4       |
//        -----------------------------------------------
//        |   SetSubVer     |   set<string> |   ?       |
//        -----------------------------------------------
//        |   Priority      |   int32       |   4       |
//        -----------------------------------------------
//        |   Comment       |   string      |   ?       |
//        -----------------------------------------------
//        |   StatusBar     |   string      |   ?       |
//        -----------------------------------------------
//        |   Reserved      |   string      |   ?       |
//        -----------------------------------------------
//        |   Total  (Fixed)                |   45      |
//        -----------------------------------------------
//
// NOTE:
//      * string is a VarString i.e VarInt length followed by the string itself
//      * set<string> is a VarInt followed by as many number of strings
//      * set<int32> is a VarInt followed by as many number of ints
//      * fixedAlertSize = 40 + 5*min(VarInt)  = 40 + 5*1 = 45
//
// Now we can define bounds on Alert size, SetCancel and SetSubVer

//fixed size of the alert payload
const fixedAlertSize = 45

//maxsignatureSize is the max size of an ecdsa signature
//note :since this size is fixed and < 255,the size of varint required = 1

const maxSignatureSize = 72

//maxalertsize is the maximum size an alert
// MessagePayload = VarInt(Alert) + Alert + VarInt(Signature) + Signature
// MaxMessagePayload = maxAlertSize + max(VarInt) + maxSignatureSize + 1
const maxAlertSize = MaxMessagePayload - maxSignatureSize - MaxVarIntPayload - 1

// maxCountSetCancel is the maximum number of cancel IDs that could possibly
// fit into a maximum size alert.
//
// maxAlertSize = fixedAlertSize + max(SetCancel) + max(SetSubVer) + 3*(string)
// for caculating maximum number of cancel IDs, set all other var  sizes to 0
// maxAlertSize = fixedAlertSize + (MaxVarIntPayload-1) + x*sizeOf(int32)
// x = (maxAlertSize - fixedAlertSize - MaxVarIntPayload + 1) / 4
const maxCountSetCancel = (maxAlertSize - fixedAlertSize - MaxVarIntPayload + 1) / 4

// maxCountSetSubVer is the maximum number of subversions that could possibly
// fit into a maximum size alert.
//
// maxAlertSize = fixedAlertSize + max(SetCancel) + max(SetSubVer) + 3*(string)
// for caculating maximum number of subversions, set all other var sizes to 0
// maxAlertSize = fixedAlertSize + (MaxVarIntPayload-1) + x*sizeOf(string)
// x = (maxAlertSize - fixedAlertSize - MaxVarIntPayload + 1) / sizeOf(string)
// subversion would typically be something like "/Satoshi:0.7.2/" (15 bytes)
// so assuming < 255 bytes, sizeOf(string) = sizeOf(uint8) + 255 = 256
const maxCountSetSubVer = (maxAlertSize - fixedAlertSize - MaxVarIntPayload + 1) / 256

// alert contains the data deserialized from the msgalert payload
type Alert struct {
	//Alert format version
	Version int32
	//timeStamp beyond which should stop relaying this alert
	RelayUntil int64

	//timestamp beyond which this alert is no longer in effect and should be ignored
	Expiration int64

	//a unique Id number for this alert
	ID int32

	//all alerts with id less than or equal to this number should cancelled ,
	//deleted and not accepted in the future
	Cancel int32

	//all alerts IDs contained in this set should be cancelled as above
	SetCancel []int32

	//this alert only applies to versions greater than or equal to this version
	// others versions shouldd still relay it.
	MinVer int32

	//this alert only applies to versions less than or equal to this version other
	//versions should still relay it.
	MaxVer int32

	//if this set contains any elements,then only nodes that have their subver contained
	//in this set are affected by the alert.other viersons should still relay it.
	SetSubVer []string

	//Relative priority compared to other alerts
	Priority int32

	//a comment on the alert that is not displayed
	Comment string

	//the alert message that is displayed to the user
	StatusBar string

	//Reserved
	Reserved string
}

//serialize encodes the alert to w using the alert protocol encoding format.
func (alert *Alert) Serialize(w io.Writer, pver uint32) error {
	err := writeElements(w, alert.Version, alert.RelayUntil,
		alert.Expiration, alert.ID, alert.Cancel)

	if err != nil {
		return err
	}

	count := len(alert.SetCancel)
	if count > maxCountSetCancel {
		str := fmt.Sprintf("too many cancel alert IDs for alert"+
			"[count%v,max%v	]", count, maxCountSetCancel)
		return messageError("alet.serize", str)
	}
	err = WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		err = writeElemnet(w, alert.SetCancel[i])
		if err != nil {
			return err
		}
	}
	err = writeElemnets(w, alert.MinVer, alert.MaxVer)
	if err != nil {
		return err
	}
	count = len(alert.SetSubVer)
	if count > maxCountSetSubVer {
		str := fmt.Sprintf("too many sub versions for alert "+
			"[count %v, max %v]", count, maxCountSetSubVer)
		return messageError("Alert.Serialize", str)
	}

	err = WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		err = WriteVarString(w, pver, alert.SetSubVer[i])
		if err != nil {
			return err
		}
	}

	err = writeElement(w, alert.Priority)
	if err != nil {
		return err
	}
	err = WriteVarString(w, pver, alert.Comment)
	if err != nil {
		return err
	}
	err = WriteVarString(w, pver, alert.StatusBar)
	if err != nil {
		return err
	}
	return WriteVarString(w, pver, alert.Reserved)

}

// deserialize decodes from r into the receiver using the alert protocol
// encoding format.
func (alert *Alert) Deserialize(r io.Reader, pver uint32) error {
	err := readElements(r, &alert.Version, &alert.RelayUntil,
		&alert.Expiration, &alert.ID, &alert.Cancel)

	if err != nil {
		return err
	}

	//setcancel :first read a varint that contains count-the number of cancel ids,
	// then iterate count times and read them
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if count > maxCountSetCancel {
		str := fmt.Sprintf("too many cancel alert IDs for alert "+
			"[count %v, max %v]", count, maxCountSetCancel)
		return messageError("Alert.Deserialize", str)
	}

	alert.SetCancel = make([]int32, count)
	for i := 0; i < int(count); i++ {
		err := readElement(r, &alert.SetCancel[i])
		if err != nil {
			return err
		}

		err = readElements(r, &alert.MinVer, &alert.MaxVer)
		if err != nil {
			return err
		}

		//setsubver:similar to setcancel
		//but read count number of sub-version strings
		count, err = ReadVarInt(r, pver)
		if err != nil {
			return err
		}
		if count > maxCountSetCancel {
			str := fmt.Sprintf("too many sub versions for alert "+
				"[count %v, max %v]", count, maxCountSetSubVer)
			return messageError("Alert.Deserialize", str)
		}

		alert.SetSubVer = make([]string, count)
		for i := 0; i < int(count); i++ {
			alert.SetSubVer[i], err = ReadVarString(r, pver)
			if err != nil {
				return err
			}
		}

	}
	err = readElement(r, &alert.Priority)
	if err != nil {
		return err
	}
	alert.Comment, err = ReadVarString(r, pver)
	if err != nil {
		return err
	}

	alert.StatusBar, err = ReadVarString(r, pver)
	if err != nil {
		return err
	}
	alert.Reserved, err = ReadVarString(r, pver)
	return err

}

//newalertfrompayload returns an alert with values deserialized form
//the serialized payload
func NewAlertFromPayload(serializedPayload []byte, pver uint32) (*Alert, error) {
	var alert Alert
	r := bytes.NewReader(serializedPayload)
	err := alert.Deserialize(r, pver)
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// msgalert implements the message interface and defines a bitcoin alert message
// this is a signed message that provides notifications that the client should
// display if the signature machtes the key.bitcoind/bitcoin-qt only checks against
// a signature from the core developers
type MsgAlert struct {

	//serializedpayload is the alert payload serialized as a string so that
	//the version can change but the alert can still be passed on by older
	//clients
	SerializedPayload []byte

	//Signature is the ecdsa signature of the message
	Singature []byte

	//deserialized payload
	Payload *Alert
}

//btcdecode decodes r using the bitcoin procotol encoding into the receiver
//this is part of the message interface implementation
func (msg *MsgAlert) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	var err error
	msg.SerializedPayload, err = ReadVarBytes(r, pver, MaxMessagePayload, "alert serialized payload")
	if err != nil {
		return err
	}
	msg.Payload, err = NewAlertFromPayload(msg.SerializedPayload, pver)
	if err != nil {
		msg.Payload = nil
	}
	msg.Singature, err = ReadVarBytes(r, pver, MaxMessagePayload, "alert signature")
	return err

}

//btcencode encodes the receiver to w using the bitcoin procotol encoding.
//this is part of message interface inplementation
func (msg *MsgAlert) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	var err error
	var seriallizedpayload []byte
	if msg.Payload != nil {
		//try to serialize payload if possible
		r := new(bytes.Buffer)
		err = msg.Payload.Serialize(r, pver)
		if err != nil {
			//serialize failed - ignore & fallback
			seriallizedpayload = msg.SerializedPayload
		} else {
			seriallizedpayload = r.Bytes()
		}

	} else {
		seriallizedpayload = msg.SerializedPayload
	}
	slen := uint64(len(seriallizedpayload))
	if slen == 0 {
		return messageError("MsgAlert.BtcEncode", "empty serialized payload")
	}
	err = WriteVarBytes(w, pver, seriallizedpayload)
	if err != nil {
		return err
	}
	return WriteVarBytes(w, pver, msg.Singature)
}

//command returns the protocol command string for the message
//this is part of the message interface implementation
func (msg *MsgAlert) Command() string {
	return CmdAlert
}

//maxpayloadlength returns the maximum length the payload can be for
//the receiver.this is part of the message interface implementation
func (msg *MsgAlert) MaxPayloadLength(pver uint32) uint32 {
	//since this can vary depengding on the message .make it the max size allowed
	return MaxMessagePayload
}

//newmsgalert returns a new bitcoin alert message  that conforms to the interface
// see msgalert for detials
func NewMsgAlert(serializedPayload []byte, signature []byte) *MsgAlert {
	return &MsgAlert{
		SerializedPayload: serializedPayload,
		Singature:         signature,
		Payload:           nil,
	}
}


//over
