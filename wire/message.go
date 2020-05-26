package wire

import (
	"bytes"
	"fmt"
	"io"
)

//messageheadersize is the number of bytes in a bitcion message header
//bitcoin network(maginc)4 bytes + command12 bytes + payload length 4 bytes
// + chencsum 4bytes

const MessageHeaderSize = 24

//commandsize is the fixed size of all commands in the comman bitcoin message
//header.shorter commands must be zero padded.

const CommandSize = 12

//maxmessagepayload is the maximum bytes a message can be regardless of other
//individual limints imposed by messages themselves.
const MaxMemessagePayload = (1024 * 1024 * 32) //32MB

//command used in the bitcoin message headers which describe the types of message
const (
	CmdVersion      = "version"
	CmdVerAck       = "verack"
	CmdGetAddr      = "getaddr"
	CmdAddr         = "addr"
	CmdGetBlocks    = "getblocks"
	CmdInv          = "inv"
	CmdGetData      = "getdata"
	CmdNotFound     = "notfound"
	CmdBlock        = "block"
	CmdTx           = "tx"
	CmdGetHeaders   = "getheaders"
	CmdHeaders      = "headers"
	CmdPing         = "ping"
	CmdPong         = "pong"
	CmdAlert        = "alert"
	CmdMemPool      = "mempool"
	CmdFilterAdd    = "filteradd"
	CmdFilterClear  = "filterclear"
	CmdFilterLoad   = "filterload"
	CmdMerkleBlock  = "merkleblock"
	CmdReject       = "reject"
	CmdSendHeaders  = "sendheaders"
	CmdFeeFilter    = "feefilter"
	CmdGetCFilters  = "getcfilters"
	CmdGetCFHeaders = "getcfheaders"
	CmdGetCFCheckpt = "getcfcheckpt"
	CmdCFilter      = "cfilter"
	CmdCFHeaders    = "cfheaders"
	CmdCFCheckpt    = "cfcheckpt"
)

//messageencoding represents the wire message encoding format to be used.
type MessageEncoding uint32

const (
	//baseencoding encodes all message in the default format specified
	// for the bitcion wire protocol
	BaseEncoding MessageEncoding = 1 << iota

	//witnessencoding encodes all message other than transation messages
	//using the defualt bitcoin wire protocol specification.for transaction
	//message.the new encoding format detailed in bip0144 will be used.
	WitnessEncoding
)

//latestencoding is the most recently specified encoding for the bitcoin wire
//protocol
var latestEncoding = WitnessEncoding

//message in an interface that describe a bitcoin message .a type that implements
//message has complete control over the representation of its data and may therefore
//contain additional or fewer fields than those which are used directly in the protocol
//encoded message
type Message interface {
	BtcDecod(io.Reader, uint32, MessageEncoding) error
	BtcEncode(io.Writer, uint32, MessageEncoding) error
	Command() string
	MaxPayloadLength(uint32) uint32
}

//makeemptymessage creates a message of the appropriate contrete type of based
// on the command

func makeEmptyMessage(command string) (Message, error) {
	var msg Message
	switch command {
	case CmdVersion:
		msg = &MsgVersion{}

	case CmdVerAck:
		msg = &MsgVerAck{}

	case CmdGetAddr:
		msg = &MsgGetAddr{}

	case CmdAddr:
		msg = &MsgAddr{}

	case CmdGetBlocks:
		msg = &MsgGetBlocks{}

	case CmdBlock:
		msg = &MsgBlock{}

	case CmdInv:
		msg = &MsgInv{}

	case CmdGetData:
		msg = &MsgGetData{}

	case CmdNotFound:
		msg = &MsgNotFound{}

	case CmdTx:
		msg = &MsgTx{}

	case CmdPing:
		msg = &MsgPing{}

	case CmdPong:
		msg = &MsgPong{}

	case CmdGetHeaders:
		msg = &MsgGetHeaders{}

	case CmdHeaders:
		msg = &MsgHeaders{}

	case CmdAlert:
		msg = &MsgAlert{}

	case CmdMemPool:
		msg = &MsgMemPool{}

	case CmdFilterAdd:
		msg = &MsgFilterAdd{}

	case CmdFilterClear:
		msg = &MsgFilterClear{}

	case CmdFilterLoad:
		msg = &MsgFilterLoad{}

	case CmdMerkleBlock:
		msg = &MsgMerkleBlock{}

	case CmdReject:
		msg = &MsgReject{}

	case CmdSendHeaders:
		msg = &MsgSendHeaders{}

	case CmdFeeFilter:
		msg = &MsgFeeFilter{}

	case CmdGetCFilters:
		msg = &MsgGetCFilters{}

	case CmdGetCFHeaders:
		msg = &MsgGetCFHeaders{}

	case CmdGetCFCheckpt:
		msg = &MsgGetCFCheckpt{}

	case CmdCFilter:
		msg = &MsgCFilter{}

	case CmdCFHeaders:
		msg = &MsgCFHeaders{}

	case CmdCFCheckpt:
		msg = &MsgCFCheckpt{}

	default:
		return nil, fmt.Errorf("unhandled command [%s]", command)
	}
	return msg, nil

}

//messageheader defines the header structure for all bitcoin protocol messagel
type messageHeader struct {
	magic    BitcoinNet // 4bytes
	command  string     // 12bytes
	lenght   uint32     // 4 bytes
	checksum [4]byte    //4 bytes
}

//readMessageheader reads a bitcion message header from r.
func readMessageHeader(r io.Reader) (int, *messageHeader, error) {

	//since readelements does not return the ammounts of bytes read
	//attmept to read the entire header into a buffer first in case
	//there is a short read so the proper amount of read bytes are
	//known.this works since the header is a fixed size
	var headerBytes [MessageHeaderSize]byte
	n, err := io.ReadFull(r, headerBytes[:])
	if err != nil {
		return n, nil, err
	}
	hr := bytes.NewReader(headerBytes[:])

	//create and populate a messageHeader struct from the raw header
	// bytes
	hdr := messageHeader{}
	var command [CommandSize]byte
	readElements(hr, &hdr.magic, &command, &hdr.lenght, &hdr.checksum)
	return n, &hdr, nil
}

// discardInput reads n bytes from reader r in chunks and discards the read
// bytes.  This is used to skip payloads when various errors occur and helps
// prevent rogue nodes from causing massive memory allocation through forging
// header length.
func discardInput(r io.Reader, n uint32) {
	maxSize := uint32(10 * 1024) //10k at a time

	numReads := n / maxSize
	bytesRemaining := n % maxSize
	if n > 0 {
		buf := make([]byte, maxSize)
		for i := uint32(0); i < numReads; i++ {
			io.ReadFull(r, buf)
		}
	}
	if bytesRemaining > 0 {
		buf := make([]byte, bytesRemaining)
		io.ReadFull(r, buf)
	}

}

//writemessage writes a bitcoin message to w including the necessary header
//information and returns the numbers of bytes written.this function is the
//same as writemessage except it also return the numbers of bytes
//written .

func WriteMessageN(w io.Writer, msg Message, pver uint32, btcnet BitcoinNet) (int, error) {
	return WriteMessageWithEncodingN(w, msg, pver, btcnet, BaseEncoding)
}

// WriteMessage writes a bitcoin Message to w including the necessary header
// information.  This function is the same as WriteMessageN except it doesn't
// doesn't return the number of bytes written.  This function is mainly provided
// for backwards compatibility with the original API, but it's also useful for
// callers that don't care about byte counts.
func WriteMessage(w io.Writer, msg Message, pver uint32, btcnet BitcoinNet) error {
	_, err := WriteMessageN(w, msg, pver, btcnet)
	return err
}

//writesmessagewithencodingn writes a bitcoin message to w including the necessary header
//information and returns the number of bytes written this function is the same as writemessage
//except it also allows the caller to specify the message encoding format to be used
// when serializing wire message
func WriteMessageWithEncodingN(w io.Writer, msg Message, pver uint32, btcnet BitcoinNet, encoding MessageEncoding) (int, error) {
	totalBytes := 0
	//enforce max command size.
	var command [CommandSize]byte
	cmd := msg.Command()
	if len(cmd) > CommandSize {
		str := fmt.Sprintf("command[%s] is too long [max %v]", cmd, CommandSize)
		return totalBytes, messageError("WriteMessage", str)
	}
	copy(command[:], []byte(cmd))

	//encode the message payload












}
