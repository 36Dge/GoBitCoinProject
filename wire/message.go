package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"
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
const MaxMessagePayload = (1024 * 1024 * 32) //32MB

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
	length   uint32     // 4 bytes
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
	readElements(hr, &hdr.magic, &command, &hdr.length, &hdr.checksum)
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

	var bw bytes.Buffer
	err := msg.BtcEncode(&bw, pver, encoding)
	if err != nil {
		return totalBytes, err
	}
	payload := bw.Bytes()
	lenp := len(payload)

	//enforce maximum overall message payload
	if lenp > MaxMessagePayload {
		str := fmt.Sprintf("message payload is too large - encoded "+
			"%d bytes, but maximum message payload is %d bytes", lenp, MaxMessagePayload)
		return totalBytes, messageError("WriteMessage", str)
	}

	//enforce maximum message payload on the message type .
	mpl := msg.MaxPayloadLength(pver)
	if uint32(lenp) > mpl {
		str := fmt.Sprintf("message payload is too large - encoded "+
			"%d bytes, but maximum message payload size for "+
			"messages of type [%s] is %d.", lenp, cmd, mpl)
		return totalBytes, messageError("WriteMessage", str)
	}

	//create header for the message
	hdr := messageHeader{}
	hdr.magic = btcnet
	hdr.command = cmd
	hdr.length = uint32(lenp)
	copy(hdr.checksum[:], chainhash.DoubleHashB(payload)[0:4])

	//encode the header for the message ,this is done to a buffer
	//rather than directly to the wirter since writeelements does
	//not return the number of bytes wirtten
	hw := bytes.NewBuffer(make([]byte, 0, MessageHeaderSize))
	writeElements(hw, hdr.magic, command, hdr.length, hdr.checksum)

	//write header
	n, err := w.Write(hw.Bytes())
	totalBytes += n
	if err != nil {
		return totalBytes, err
	}

	//only write the payload if there is one ,e,g ,verack message do not
	//have one.
	if len(payload) > 0 {
		n, err = w.Write(payload)
		totalBytes += n
	}

	return totalBytes, err

}

//readmessagewithecoding reads,validates ,and parses the next bitcoin
//message form r for the provided protocol version and bitcoin newwork,it retruns
//the numbers of bytes read in addition to the parsed message and raw bytes
//which comprise the message .this function is the same as readmessagen except
//it allows the caller to specify which message encoding is to consult hwen
// decoding wire message
func ReadMessageWithEncodingN(r io.Reader, pver uint32, btcnet BitcoinNet,
	enc MessageEncoding) (int, Message, []byte, error) {
	totalBytes := 0
	n, hdr, err := readMessageHeader(r)
	totalBytes += n
	if err != nil {
		return totalBytes, nil, nil, err
	}

	//enforce maximum message payload
	if hdr.length > MaxMessagePayload {
		str := fmt.Sprintf("message payload is too large - header "+
			"indicates %d bytes, but max message payload is %d "+
			"bytes.", hdr.length, MaxMessagePayload)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	//check for messages from the wrong bitcoin newwork
	if hdr.magic != btcnet {
		discardInput(r, hdr.length)
		str := fmt.Sprintf("message from other network [%v]", hdr.magic)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	//check for malformed commands
	command := hdr.command
	if !utf8.ValidString(command) {
		discardInput(r, hdr.length)
		str := fmt.Sprintf("invalid command %v", []byte(command))
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	//create struct of appropriate message type based on the command.
	msg, err := makeEmptyMessage(command)
	if err != nil {
		discardInput(r, hdr.length)
		return totalBytes, nil, nil, messageError("ReadMessage", err.Error())
	}

	//check for maximun length based on the message type as a malicious cleents
	//could otherwise create a well-formed header and set the length to
	//max numbers in order to exhaust the machine,s memory.

	mpl := msg.MaxPayloadLength(pver)
	if hdr.length > mpl {
		discardInput(r, hdr.length)
		str := fmt.Sprintf("payload exceeds max length - header "+
			"indicates %v bytes, but max payload size for "+
			"messages of type [%v] is %v.", hdr.length, command, mpl)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	//read payload
	payload := make([]byte, hdr.length)
	n, err = io.ReadFull(r, payload)
	totalBytes += n
	if err != nil {
		return totalBytes, nil, nil, err
	}

	//test checksum.
	checksum := chainhash.DoubleHashB(payload)[0:4]
	if !bytes.Equal(checksum[:], hdr.checksum[:]) {
		str := fmt.Sprintf("payload checksum failed - header "+
			"indicates %v, but actual checksum is %v.",
			hdr.checksum, checksum)
		return totalBytes, nil, nil, messageError("ReadMessage", str)
	}

	//unmarshal message ,note :this must be a *bytes.buffer since
	//the msgversion btcdecode function requires it.
	pr := bytes.NewBuffer(payload)
	err = msg.BtcDecod(pr, pver, enc)
	if err != nil {
		return totalBytes, nil, nil, err
	}
	return totalBytes, msg, payload, nil
}

//readmessagen reads,validates ,and parses the next bitcoin message from f
//for the provided protocol version and bitcion network.it returns the number
//of bytes read in addition to the parsed message and raw bytes which comprise
//the message,this funcions is the same as readmessage except it also returns
//the number of bytes read.
func ReadMessageN(r io.Reader,pver uint32,btcnet BitcoinNet)(int,Message,[]byte,error){
	return ReadMessageWithEncodingN(r, pver, btcnet, BaseEncoding)
}

// ReadMessage reads, validates, and parses the next bitcoin Message from r for
// the provided protocol version and bitcoin network.  It returns the parsed
// Message and raw bytes which comprise the message.  This function only differs
// from ReadMessageN in that it doesn't return the number of bytes read.  This
// function is mainly provided for backwards compatibility with the original
// API, but it's also useful for callers that don't care about byte counts.
func ReadMessage(r io.Reader, pver uint32, btcnet BitcoinNet) (Message, []byte, error) {
	_, msg, buf, err := ReadMessageN(r, pver, btcnet)
	return msg, buf, err
}
