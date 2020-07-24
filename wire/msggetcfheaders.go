package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"io"
)

//msggetcfheaders is a message similar to msggetheaders but for commited
//filter headers ,it allows to set the filtertype field to get headers int
//the chain of basic or extended headers
type MsgGetCFHeaders struct {
	FilterType FilterType
	StartHeight uint32
	StopHash chainhash.Hash
}

//btcdecode decodes r using the bitcoin protocol encoding into the receiver
//this is part of the message interface implementation.
func(msg *MsgGetCFHeaders)BtcDecode(r io.Reader,pver uint32,_ MessageEncoding) error{
	err := readElement(r,&msg.FilterType)
	if err != nil{
		return err
	}

	err = readElement(r,&msg.StartHeight)
	if err != nil{
		return err
	}

	return readElement(r,&msg.StopHash)
}

//btcencode encodes the recvier to w using the bitcoin protocol encoding
//this is part the message interface implementation.
func(msg *MsgGetCFHeaders)BtcEncode(w io.Writer,pver uint32 ,_ MessageEncoding)error{
	err := writeElement(w,msg.FilterType)
	if err != nil{
		return err
	}

	err = writeElement(w,&msg.StartHeight)
	if err != nil{
		return err
	}

	return writeElement(w,&msg.StopHash)
}


//command returns the protocol command string for the message this is part
//of the message interface implementation.
func (msg *MsgGetCFHeaders) Command()string {
	return CmdGetCFHeaders
}

//maxpayloadlength returns the maximum length the payload can be for the
//receiver this is part of the message interface implementation.
func (msg *MsgGetCFHeaders) MaxPayloadLength(pver uint32)uint32	 {
	//filter type + uint32 + block hash
	return 1 + 4 + chainhash.HashSize

}

// NewMsgGetCFHeaders returns a new bitcoin getcfheader message that conforms to
// the Message interface using the passed parameters and defaults for the
// remaining fields.
func NewMsgGetCFHeaders(filterType FilterType, startHeight uint32,
	stopHash *chainhash.Hash) *MsgGetCFHeaders {
	return &MsgGetCFHeaders{
		FilterType:  filterType,
		StartHeight: startHeight,
		StopHash:    *stopHash,
	}
}

//over

















