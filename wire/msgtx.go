package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"strconv"
)

const (

	//txversion is the current latest supported transaction version.
	TxVersion = 1

	//MaxTxINsequencenum is the maximun sequenece number the sequence field of a transaction input can be
	MaxTxInSequenceNum uint32 = 0xffffffff

	//maxperouIndex is the maximum index the index field of a previous outpoint can be
	MaxPrevOutIndex uint32 = 0xffffffff

	//SequenceLocktimedisabled is a flag that if set on a transaction input is sequence number,
	//the sequence number will not be interpreted as a relative locktime .
	SequenceLockTimeDisabled = 1 << 31

	//sequencelocktimeis second is a flag that is set on a transaction input,s sequence number,
	//the relative locktime has units of 512 seconds.
	SequenceLockTimeIsSeconds = 1 << 22

	//sequenceLocktimemask is a mask that extracts the relative locktime
	//when masked against the transaction input sequence number.
	SequenceLockTimeMask = 0x0000ffff

	//sequencelocktimegranularity is the defined time based grunularity
	//for seconds-based relative time locks.when conversing from seconds
	//to a sequence number.the value is right shifted by this amount .
	//therefore the granularity of relative time locks in 512 or 2^9 seconds
	//enforced ralative lock times are multiples of 512 seconds .
	SequenceLockTimeGranularity = 9

	//defaultTxInoutalloc is the default size used for the backing array for
	//transaction inputs and outputs, the array will dynaminclly grow as needed
	//but this figure is intended to provided enough space for the number of
	//inputs and outputs in a typical transaction without needing to grow the
	//backing arrya multiple times.
	defualtTxInOutAlloc = 15

	//mintxinpayload is the minimum payload size for a transacton input
	//previousoutpoint.hash + previousputpoint.index 4 bytes + varint for
	//signaturescript length 1 byte + sequece 4 bytes .
	minTxInPayload = 9 + chainhash.HashSize

	//maxtxinpermessage is the maximum number of transactions inputs that a
	//transaction which fits into message could possible have.
	maxTxInPerMessage = (MaxMessagePayload / minTxInPayload) + 1

	//mintxoutpayload is the minimum payload size for a transaction output.
	//value 8bytes + varint for pkscript length 1 byte.
	MinTxOutPayload = 9

	// maxTxOutPerMessage is the maximum number of transactions outputs that
	// a transaction which fits into a message could possibly have.
	maxTxOutPerMessage = (MaxMessagePayload / MinTxOutPayload) + 1

	//mintxpayload is the minimum payload size for a transaction .note
	//that any realistically usable transaction must have at least one
	//input or output . but that is a rule enforced at a higher layer, so
	// it is intentionally not included here.
	// Version 4 bytes + Varint number of transaction inputs 1 byte + Varint
	// number of transaction outputs 1 byte + LockTime 4 bytes + min input
	// payload + min output payload.

	minTxPayload = 10

	//frelistmaxscriptsize is the size of each buffer in the free list
	//that is used for deserializing scripts from the wire before they are
	//concatenated into a single contigous buffers this value was choesen
	//because it is slightly more than twice the size of the vast majority
	// of all "standard" scripts.  Larger scripts are still deserialized
	// properly as the free list will simply be bypassed for them.

	freeListMaxScriptSize = 512

	// freeListMaxItems is the number of buffers to keep in the free list
	// to use for script deserialization.  This value allows up to 100
	// scripts per transaction being simultaneously deserialized by 125
	// peers.  Thus, the peak usage of the free list is 12,500 * 512 =
	// 6,400,000 bytes.
	freeListMaxItems = 12500

	// maxWitnessItemsPerInput is the maximum number of witness items to
	// be read for the witness data for a single TxIn. This number is
	// derived using a possble lower bound for the encoding of a witness
	// item: 1 byte for length + 1 byte for the witness item itself, or two
	// bytes. This value is then divided by the currently allowed maximum
	// "cost" for a transaction.
	maxWitnessItemsPerInput = 500000

	// maxWitnessItemSize is the maximum allowed size for an item within
	// an input's witness data. This number is derived from the fact that
	// for script validation, each pushed item onto the stack must be less
	// than 10k bytes.
	maxWitnessItemSize = 11000
)

//witnessmakerbytes are a pair of bytes specific to the witness encoding
//if this sequence is encoutered. then it indicates a transaction has witness
//data the first byte is an alway ox00 maker byte.which allows decoder to
//distinguish a serialized transaction with witness from regular (legacy)
// one. The second byte is the Flag field, which at the moment is always 0x01,
// but may be extended in the future to accommodate auxiliary non-committed
var witnessMarkerBytes = []byte{0x00, 0x01}

//scriptFreeList defines a free list of byte slices (up to the maximum number
// defined by the freeListMaxItems constant) that have a cap according to the
// freeListMaxScriptSize constant.  It is used to provide temporary buffers for
// deserializing scripts in order to greatly reduce the number of allocations
// required.
//
// The caller can obtain a buffer from the free list by calling the Borrow
// function and should return it via the Return function when done using it.

type scriptFreeList chan []byte

//borrow returns a byte silice from the free list with a length according the
//provided size.a new buffer is allocated if there are any items availalbe .

//when the size if larger than the max size allowed for items on the free list
//a new buffer of the appropriate size is allocated and returned it is safe
//to attempt to retrun said buffer via the retrun function as it will be ignored
//and allowed to go the garbage collector.
func (c scriptFreeList) Borrow(size uint64) []byte {
	if size > freeListMaxScriptSize {
		return make([]byte, size)
	}
	var buf []byte
	select {
	case buf = <-c:
	default:
		buf = make([]byte, freeListMaxScriptSize)
	}
	return buf[:size]
}

//return puts the provided byte slice back on the free list when it has
//a cap of the expected length.the buffer is expected to have been obtained
//via the borrow function.any slices that are not of the approriate size .
//such as those whose size if greater than the largest allowed free list item
//size are simply ignodred so they can go to the garbage colletcor.
func (c scriptFreeList) Return(buf []byte) {
	//ignore any buffers returned that are,t the expected size for the free list.
	if cap(buf) != freeListMaxScriptSize {
		return
	}
	//return the buffer to the free list when it is not full. otherwise let
	//it can be collected .
	select {
	case c <- buf:
	default:
		//let it go to the grabage colletcor.

	}
}

//create the concurrent safe free list to use for script deserialiation .as
//previously described.this free list is maintained to signnificantly reduce
//the number of allocations.
var scriptPool scriptFreeList = make(chan []byte, freeListMaxItems)

//outpoint defines a bitcoin data type that is used to track previous
//transaction outputs.

type OutPoint struct {
	Hash  chainhash.Hash
	Index uint32
}

//newoutPoint returns a new bitcoin transaction outpoint point with the
//provided hash and index
func NewOutPoint(hash *chainhash.Hash, index uint32) *OutPoint {
	return &OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

//string returns the outpoint in the human-readable form"hash:index".
func (o OutPoint) String() string {
	//allocate enough for hash string ,colon and 10 digits .although
	//at the time of wirting .the number of digits can be no greater than
	//the length of the decimal representation of maxtxoutpermessage .the
	//maxium message palload may increase in the future and this optimization
	//may go unnoticed .so allocate space for 10 decial digits .which will fit
	//any uint32.
	buf := make([]byte, 2*chainhash.HashSize+1, 2*chainhash.HashSize+1+10)
	copy(buf, o.Hash.String())
	buf[2*chainhash.HashSize] = ':'
	buf = strconv.AppendUint(buf, uint64(o.Index), 10)
	return string(buf)
}

//txin defines a bitcoin transaction input.
type TxIn struct {
	PreviousOutPoint OutPoint
	SignatureScript  []byte
	Witness          TxWitness
	Sequence         uint32
}

//serializesize returns the number of bytes it would take to serialize to the
//the transaction input.
func (t *TxIn) SerializeSize() int {
	//outpoint hash 32bytes + outpoint index 4 bytes + sequence 4 bytes +
	//serialize varint size for the length of singnature signatureScript +
	//signaturescript bytes.
	return 40 + VarIntSerializeSize(uint64(len(t.SignatureScript))) + len(t.SignatureScript)
}

//newtxin returns a new bitcoin transaction input with the provided previous
//outpoint and signature script with a default sequence of maxtxinsequencenum.
func NewTxIn(prevOut *OutPoint, signatureScript []byte, witness [][]byte) *TxIn {
	return &TxIn{
		PreviousOutPoint: *prevOut,
		SignatureScript:  signatureScript,
		Witness:          witness,
		Sequence:         MaxTxInSequenceNum,
	}
}

//txwietness defines the witness for a txin .a witness is to be inter interperted as
//a slice of byte slices,or a stack with one or many elements.
type TxWitness [][]byte

//serializesize returns the numbers of bytes it would take to serilaize the
//transaction inputs witness
func (t TxWitness) SerializeSize() int {
	//a varint to signal the number of elements the witness has.

	n := VarIntSerializeSize(uint64(len(t)))

	//for each element in the witness.we will need a varint to signal the
	//size of the element .then finally the number of bytes the element
	//iteselt comprises.
	for _, witItem := range t {
		n += VarIntSerializeSize(uint64(len(witItem)))
		n += len(witItem)
	}
	return n
}

//txout defines a bitcoin transaction output.
type TxOut struct {
	Value    int64
	PkScript []byte
}

//serializesize returns the number of bytes it would take to serialize the
//the transaction output

func (t *TxOut) SerializeSize() int {
	//value 8 bytes + serialzed varint size for the length of PKscript + Pkscript bytes
	return 8 + VarIntSerializeSize(uint64(len(t.PkScript))) + len(t.PkScript)

}

//newtxout returns a new bitcoin transaction outputs with the provoded transaction
//value and public key script 
func NewTxOut(value int64, pkScript []byte) *TxOut {
	return &TxOut{
		Value:    value,
		PkScript: pkScript,
	}
}

//msgtx implements the message interface and represents a bitcoin tx message
//it is used to deliver transaction information in response to a getdata
//messae for a given transaction .

//use the addTxin and addtxout functions to build up the list of transaction
//inputs and outpus.
type MsgTx struct {
	Version  int32
	TxIn     []*TxIn
	TxOut    []*TxOut
	LockTime uint32
}

//addtxin adds a transaction input to the message
func (msg *MsgTx) AddTxIn(ti *TxIn) {
	msg.TxIn = append(msg.TxIn, ti)
}

//addtxout adds a transaction output to the message
func (msg *MsgTx) AddTxOut(to *TxOut) {
	msg.TxOut = append(msg.TxOut, to)
}







