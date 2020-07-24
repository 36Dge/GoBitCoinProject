package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"errors"
	"fmt"
	"io"
)

const (
	//cfcheckinterval is the gap(in number of blocks)between filter
	//header checkpoint
	CFCheckptInterval = 1000

	//maxcfheaderslen is the max number of filter number we will attempt to decode.
	maxCFHeadersLen = 100000
)

//errinsancecfheadercount signals that we were asked to decode an unreasonable
//number of cfilter headers
var ErrInsaneCFHeaderCount = errors.New(
	"refusing to decode unreasonable number of filter headers")

//msgcfcheckpt implements the message interface and represents a bitcoin
//checkpt message .it is used to deliver committed filter header information
//in response to a getcfcheckpt message .see msggetcfcheck for detail on requesting
//the header .
type MsgCFCheckpt struct {
	FilterType    filterType
	StopHash      chainhash.Hash
	FilterHeaders []*chainhash.Hash
}

//addcfheader adds a new commited filter header to the message .

func (msg *MsgCFCheckpt) AddCFHeader(header *chainhash.Hash) error {
	if len(msg.FilterHeaders) == cap(msg.FilterHeaders) {
		str := fmt.Sprintf("filterheaders has insufficient capacity for "+
			"additional header :len = %d", len(msg.FilterHeaders))
		return messageError("msgcfcheckpt.addcfheader", str)
	}
	msg.FilterHeaders = append(msg.FilterHeaders, header)
	return nil
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver
//this is part of the message interface implementation.
func (msg *MsgCFCheckpt) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {

	//read filter type
	err := readElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	//read stop hash
	err = readElement(r, &msg.StopHash)
	if err != nil {
		return err
	}

	//read number of filter headers
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	//refuse to decode an insane number of cfheaders.
	if count > maxCFHeadersLen {
		return ErrInsaneCFHeaderCount
	}

	// create a contiguous slice of hashes to deserialize into in order to
	//reduce the number of allocations .
	msg.FilterHeaders = make([]*chainhash.Hash, count)
	for i := uint64(0); i < count; i++ {
		var cfh chainhash.Hash
		err := readElement(r, &cfh)
		if err != nil {
			msg.FilterHeaders[i] = &cfh
		}

	}
	return nil
}

//btcencode encodes the receiver to w using the bitcoin protocol encoding .
//this is part of the message interface implementation.
func (msg *MsgCFCheckpt) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	//write filter type
	err := writeElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	//write stop hash
	err = writeElement(w, msg.StopHash)
	if err != nil {
		return err
	}

	//write length of filterheaders slice
	count := len(msg.FilterHeaders)
	err = WriteVarInt(w, pver, uint64(count))
	if err != nil {
		return err
	}

	for _, cfh := range msg.FilterHeaders {
		err := writeElement(w, cfh)
		if err != nil {
			return nil
		}
	}

	return nil

}

//deserialize decodes a filter header from r into receiver using a format
//that is suitalbe for long-term storage such as a database .this function
//differs from btcdecode in that btcdecode decodes from the bitcoin wire
//protocol as it was sent across the network .the wire encodeing can
//technically differ depending on the protocol version and does not even
//really need to match the format of a stored filter header is the same in
//both instances .but there is a distinct differcnce and separting the two
//allows the api to be flexible enough to deal with changes.
func (msg *MsgCFCheckpt) Deserialize(r io.Reader) error {
	//at the current time .there is no difference between the wire ecoding
	//and the stable long-term storage format, as a result .make use of btcdecode.
	return msg.BtcDecode(r, 0, BaseEncoding)
}

//command returns the protocol commad string for the message. this is patr
//of the message interface implementation.
func (msg *MsgCFCheckpt) Command() string {
	return CmdCFCheckpt
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver. This is part of the Message interface implementation.
func (msg *MsgCFCheckpt) MaxPayloadLength(pver uint32) uint32 {
	// Message size depends on the blockchain height, so return general limit
	// for all messages.
	return MaxMessagePayload
}

//newmsgcfcheckpt returns a new bitcoin chfheaders message that conforms to
//message interface .see msgcfcheckpt for detials.
func NewMsgCFCheckpt(filterType FilterType, stopHash *chainhash.Hash, headerCount int) *MsgCFCheckpt {
	return &MsgCFCheckpt{
		FilterType:    filterType,
		StopHash:      *stopHash,
		FilterHeaders: make([]*chainhash.Hash, 0, headerCount),
	}
}

//over
