package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"math/big"
	"time"
)

//blockstatus is bit field representing the validation state of the block
type blockStatus byte

const (
	//statusdatastored indicate that the block payload is stored on disk
	statusDataStored blockStatus = 1 << iota

	//statusvalid indicates that the block have been fully validted.
	statusValid

	//sattusinvalidancestor indicates that one of the block ancestors has
	//has failed validation. thus the block is also invalid.
	statusInvalidAncestor

	//statusnone indicates that the block has no validation state
	//flags set.
	statusNone blockStatus = 0
)

// HaveData returns whether the full block data is stored in the database. This
// will return false for a block node where only the header is downloaded or
// kept.
func (status blockStatus) HaveData() bool {
	return status&statusDataStored != 0
}

// KnownValid returns whether the block is known to be valid. This will return
// false for a valid block that has not been fully validated yet.
func (status blockStatus) KnownValid() bool {
	return status&statusValid != 0
}

// KnownInvalid returns whether the block is known to be invalid. This may be
// because the block itself failed validation or any of its ancestors is
// invalid. This will return false for invalid blocks that have not been proven
// invalid yet.
func (status blockStatus) KnownInvalid() bool {
	return status&(statusValidateFailed|statusInvalidAncestor) != 0
}

// blockNode represents a block within the block chain and is primarily used to
// aid in selecting the best chain to be the main chain.  The main chain is
// stored into the block database.
type blockNode struct {
	// NOTE: Additions, deletions, or modifications to the order of the
	// definitions in this struct should not be changed without considering
	// how it affects alignment on 64-bit platforms.  The current order is
	// specifically crafted to result in minimal padding.  There will be
	// hundreds of thousands of these in memory, so a few extra bytes of
	// padding adds up.

	// parent is the parent block for this node.
	parent *blockNode

	// hash is the double sha 256 of the block.
	hash chainhash.Hash

	// workSum is the total amount of work in the chain up to and including
	// this node.
	workSum *big.Int

	// height is the position in the block chain.
	height int32

	// Some fields from block headers to aid in best chain selection and
	// reconstructing headers from memory.  These must be treated as
	// immutable and are intentionally ordered to avoid padding on 64-bit
	// platforms.
	version    int32
	bits       uint32
	nonce      uint32
	timestamp  int64
	merkleRoot chainhash.Hash

	// status is a bitfield representing the validation state of the block. The
	// status field, unlike the other fields, may be written to and so should
	// only be accessed using the concurrent-safe NodeStatus method on
	// blockIndex once the node has been added to the global index.
	status blockStatus
}

//Initblocknode intiializes a block node from the given header
//and parent node. calculating the height and worksum from the respective
//fields on the parent this func is not safe for concurrrent access .it
//must be called when inttially creating a node.
func initBlockNode(node *blockNode, blockHeader *wire.BlockHeader, parent *blockNode) {

	*node = blockNode{
		hash:       blockHeader.BlockHash(),
		workSum:    Calcwork(blockHeader.Bits),
		version:    blockHeader.Version,
		bits:       blockHeader.Bits,
		nonce:      blockHeader.Nonce,
		timestamp:  blockHeader.Timestamp.Unix(),
		merkleRoot: blockHeader.MerkleRoot,
	}

	if parent != nil {
		node.parent = parent
		node.height = parent.height + 1
		node.workSum = node.workSum.Add(parent.workSum, node.workSum)

	}

}

// newBlockNode returns a new block node for the given block header and parent
// node, calculating the height and workSum from the respective fields on the
// parent. This function is NOT safe for concurrent access.
func newBlockNode(blockHeader *wire.BlockHeader, parent *blockNode) *blockNode {
	var node blockNode
	initBlockNode(&node, blockHeader, parent)
	return &node
}

//header constructs a block header from the node and returns it.
//this function is safe for concurrent access .
func (node *blockNode) Header() wire.BlockHeader {
	//no lock is needed because all accessed fields are immutable.
	prevHash := &zeroHash
	if node.parent != nil {
		prevHash = &node.parent.hash
	}

	return wire.BlockHeader{
		Version:    node.version,
		PrevBlock:  *prevHash,
		MerkleRoot: node.merkleRoot,
		Timestamp:  time.Unix(node.timestamp, 0),
		Bits:       node.bits,
		Nonce:      node.nonce,
	}
}

//ancestor returns the ancestor block node at the provided height by
//following the chain backwards from this node .the returned block will
//be nil when a hegiht is required that is after the hight of passed node
//or is less than zero.
func (node *blockNode)Ancestor(height int32) *blockNode  {
	if height < 0 || height > node.height {
		return nil
	}
	n := node
	for;n!= nil&& n.height != height; n = n.parent{
		// Intentionally left blank
	}
	return n
}
//relativeancetor returns the ancestor block node a relative distance blcoks
//before this node .this is equtivalent to calling ancestor with the node
//is height minus provided disatance .
//this function is safe for concureent access.
func (node *blockNode)RelativeAncestor (distance int32)*blockNode  {
	return node.Ancestor(node.height - distance)
}











