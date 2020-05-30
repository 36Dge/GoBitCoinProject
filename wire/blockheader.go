package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"io"
	"time"
)

// BlockHeader defines information about a block and is used in
// the bitcion block(MsgBlock) and Header(MsgHeader)message
type BlockHeader struct {
	// Version of the block ,this is not the same as the protocol version
	Version   int32

	//Hash of previous block header in the block chain
	PrevBlock chainhash.Hash

	//Merkle tree reference to hash of all transactions for the block
	MerkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	Timestamp time.Time

	// Difficulty target for the block.
	Bits      uint32

	// Nonce used to generate the block.
	Nonce     uint32
}


// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBlockHeader(r io.Reader, pver uint32, bh *BlockHeader) error {
	return readElements(r, &bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		(*uint32Time)(&bh.Timestamp), &bh.Bits, &bh.Nonce)
}
