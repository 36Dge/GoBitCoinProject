package wire

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"io"
)

const (

	// maxinvpersms is the maximum number of inventory vectors that can be in
	// a single bitcoin inv message.

	MaxInvPerMsg = 50000

	//maximum payload size for an inventory vector
	maxInvVectPayload = 4 + chainhash.HashSize

	// InvWitnessflag denotes that the inventory vector type is requesting
	// or sending a version which includes witness data.
	InWitnessFlag = 1 << 30
)

// invtype represents the allowed types of inventory .See invvect

type InvType uint32

// These constants define the various supported inventory vector types
const (
	InvTypeError                InvType = 0
	InvTypeTx                   InvType = 1
	InvTypeBlock                InvType = 2
	InvTypeFilteredBlock        InvType = 3
	InvTypeWitnessBlock         InvType = InvTypeBlock | InvWitnessFlag
	InvTypeWitnessTx            InvType = InvTypeTx | InvWitnessFlag
	InvTypeFilteredWitnessBlock InvType = InvTypeFilteredBlock | InvWitnessFlag
)

// Map of service flags back to their constant names for pretty print
var ivStrings = map[InvType]string{

	InvTypeError:                "ERROR",
	InvTypeTx:                   "MSG_TX",
	InvTypeBlock:                "MSG_BLOCK",
	InvTypeFilteredBlock:        "MSG_FILTERED_BLOCK",
	InvTypeWitnessBlock:         "MSG_WITNESS_BLOCK",
	InvTypeWitnessTx:            "MSG_WITNESS_TX",
	InvTypeFilteredWitnessBlock: "MSG_FILTERED_WITNESS_BLOCK",
}

// string returns the invtype in human-readable form
func (invtype InvType) String() string {
	if s, ok := ivStrings[invtype]; ok {
		return s
	}
	return fmt.Sprintf("unknown invtype (%d)", uint32(invtype))
}

//invvect defines a bitcoin inventory vector which is used to describe data
//as specified by the type field ,that a peer wants ,has,or does not have another peer

type InvVect struct {
	Type InvType        // type of data
	Hash chainhash.Hash // hash of the data
}

//newinvvect returns a new invvect using the provided type and hash
func NewInvVect(typ InvType, hash *chainhash.Hash) *InvVect {
	return &InvVect{
		Type: typ,
		Hash: *hash,
	}
}

//readinvvect reads an encoded invvect from r depending on the protocol version
func readInvVect(r io.Reader, pver uint32, iv *InvVect) error {
	return readElements(r, &iv.Type, &iv.Hash)
}

//writeinvvect serializes an invvect to w depending on the protocol version
func writeInvVect(w io.Writer, pver uint32, iv *InvVect) error {
	return writeElements(w, iv.Type, &iv.Hash)
}

//over