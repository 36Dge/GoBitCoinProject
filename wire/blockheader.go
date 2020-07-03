package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"bytes"
	"io"
	"time"
)

//maxblockheaderpayload is the maximum number of bytes a blcok header can be
//version 4 bytes timestamp 4bytes + bit 4bytes + nonce 4bytes + prevblock and
//merkleroot hashes
const MaxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

// BlockHeader defines information about a block and is used in
// the bitcion block(MsgBlock) and Header(MsgHeader)message
type BlockHeader struct {
	// Version of the block ,this is not the same as the protocol version
	Version int32

	//Hash of previous block header in the block chain
	PrevBlock chainhash.Hash

	//Merkle tree reference to hash of all transactions for the block
	MerkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	Timestamp time.Time

	// Difficulty target for the block.
	Bits uint32

	// Nonce used to generate the block.
	Nonce uint32
}

//blockhaderlen is a constant that repeesents the number of bytes for a block
//header.

const blockHeaderLen = 80

//blockhash computes the block indetifier hash for the given block haders
func (h *BlockHeader) BlockHash() chainhash.Hash {

	//encode the header and double sha 256 everything prior to the number of
	//transaction ignore the error returns since there is no way the encode could
	//fail except being out of memory which would cause a run_time panic
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockPayload))
	_ = writeBlockHeader(buf, 0, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

//btcdecode decdes r using the bitcoin protocol encoding into the receiver
//this is part of the message interface implementation. see deserialize for
//decoding block haders stored to disk ,such as in a database,as opposed to
//decoding block haders from the wire.
func (h *BlockHeader) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {

	return readBlockHeader(r, pver, h)
}

//btcencode encodes the receiver to w using the bitcoin protocol encoding
//this is part of the meesage interface implementation. see serialize for
//encoding block haders to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BlockHeader) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	return writeBlockHeader(w, pver, h)
}


//deserialize decodes a block header from r into the receiver using a format
//that is suitable for long-term storage such as a database while respecting
//the version field.
func(h *BlockHeader)Deserialize(r io.Reader)error{
	//at the current time .there is no difference between the wire cncoding
	//at protocol version0 and the stable long-term storage format.as a reslut
	//make use of the readblockheader.
	return readBlockHeader(r,0,h)
}

// Serialize encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BlockHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of writeBlockHeader.
	return writeBlockHeader(w, 0, h)
}


// NewBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBlockHeader(version int32, prevHash, merkleRootHash *chainhash.Hash,
	bits uint32, nonce uint32) *BlockHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BlockHeader{
		Version:    version,
		PrevBlock:  *prevHash,
		MerkleRoot: *merkleRootHash,
		Timestamp:  time.Unix(time.Now().Unix(), 0),
		Bits:       bits,
		Nonce:      nonce,
	}
}


// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBlockHeader(r io.Reader, pver uint32, bh *BlockHeader) error {
	return readElements(r, &bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		(*uint32Time)(&bh.Timestamp), &bh.Bits, &bh.Nonce)
}

// writeBlockHeader writes a bitcoin block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBlockHeader(w io.Writer, pver uint32, bh *BlockHeader) error {
	sec := uint32(bh.Timestamp.Unix())
	return writeElements(w, bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		sec, bh.Bits, bh.Nonce)
}

//over
