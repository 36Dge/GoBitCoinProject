package wire

import (
	"fmt"
	"io"
)

// BloomUpdateType specifies how the filter is updated when a match is found
type BloomUpdateType uint8

const (

	//bloomupdatenone indicates the filter is not adjusted when a match is
	//found.
	BloomUpdateNone BloomUpdateType = 0

	//bloomupdateall indicates if the filter mathes any data element in a
	//public key script .the outpoint is serialized and inserted into the
	//fliter.
	BloomUpdateAll BloomUpdateType = 1

	//bloomupdatep2pubkeyonly indicates if the filter matchs a data
	//element in a public key script and the script is of the standard
	//pay-to-pubkey or multisig. the outpoint is serilaized and inserted
	//into the filter.
	BloomUpdateP2PubkeyOnly BloomUpdateType = 2
)

const (

	//maxfilterloadhashfuncs is the maximum number of hash functions to
	//load into the bloom filter.
	MaxFilterLoadHashFuncs = 50

	//maxfilterloadfiltersize is the maximum size in bytes a filter may be
	MaxFilterLoadFilterSize = 36000
)

//msgfilterload implemnets the message interface and represents a bitcoin
//filterload message which is used to reset a bloom filter

//this message was not added until protocol version bip0037version
type MsgFilterLoad struct {
	Filter    []byte
	HashFuncs uint32
	Tweak     uint32
	Flags     BloomUpdateType
}

//btcdecode decodes r using the bitcoin portocol encoding into th receiver .
//this is part of the message interface implementation.
func (msg *MsgFilterLoad) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	if pver < BIP0037Version {
		str := fmt.Sprintf("filterload message invalid for protocol "+
			"version %d", pver)
		return messageError("msgfilterload.btcdecode", str)
	}

	var err error
	msg.Filter, err = ReadVarBytes(r, pver, MaxFilterLoadFilterSize, "filterload filter size")
	if err != nil {
		return err
	}

	err = readElements(r, &msg.HashFuncs, &msg.Tweak, &msg.Flags)
	if err != nil {
		return err
	}

	if msg.HashFuncs > MaxFilterLoadHashFuncs {
		str := fmt.Sprintf("too many filter hash functions for message"+
			"[count %v ,max %v]", msg.HashFuncs, MaxFilterLoadHashFuncs)
		return messageError("msgfilterload.btcdecode", str)
	}
	return nil

}

//btcencode encodes the receiver to w using the bitcoin portocol encoding .
//this is part of the message interface implementation.
func (msg *MsgFilterLoad) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	if pver < BIP0037Version {
		str := fmt.Sprintf("filterload message invalid for portocol "+
			"version %d", pver)
		return messageError("msgfilterload.btcencode", str)
	}

	size := len(msg.Filter)
	if size > MaxFilterLoadFilterSize {
		str := fmt.Sprintf("filterload filter size too large for message "+
			"[size %v, max %v]", size, MaxFilterLoadFilterSize)
		return messageError("MsgFilterLoad.BtcEncode", str)
	}

	if msg.HashFuncs > MaxFilterLoadHashFuncs {
		str := fmt.Sprintf("too many filter hash functions for message "+
			"[count %v, max %v]", msg.HashFuncs, MaxFilterLoadHashFuncs)
		return messageError("MsgFilterLoad.BtcEncode", str)
	}

	err := WriteVarBytes(w, pver, msg.Filter)
	if err != nil {
		return err
	}

	return writeElements(w, msg.HashFuncs, msg.Tweak, msg.Flags)

}

//maxpayloadlength returns the maximmum length the paylaod can be for the
//recevier .this is part of the message interface implementation.
func (msg *MsgFilterLoad)MaxPayloadLength (pver uint32)uint32 {
	//num filter bytes (varint) + filter + 4 bytes hash funcs +
	//4 bytes tweak + 1 byte flags.
	return uint32(VarIntSerializeSize(MaxFilterLoadFilterSize))+
		MaxFilterLoadFilterSize + 9
}

//newmsgfilterload returns a new bitcion filterload message that conforms to
//the message interface .see msgfilterload for details.
func NewMsgFilterLoad(filter []byte, hashFuncs uint32, tweak uint32, flags BloomUpdateType) *MsgFilterLoad {
	return &MsgFilterLoad{
		Filter:    filter,
		HashFuncs: hashFuncs,
		Tweak:     tweak,
		Flags:     flags,
	}
}

//over



















