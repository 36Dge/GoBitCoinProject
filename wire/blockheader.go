package wire

import (
	"BtcoinProject/chaincfg/chainhash"
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
