package blockchain

import "BtcoinProject/chaincfg"

// Checkpoints returns a slice of checkpoints (regardless of whether they are
// already known).  When there are no checkpoints for the chain, it will return
// nil.
//
// This function is safe for concurrent access.
func (b *BlockChain) Checkpoints() []chaincfg.Checkpoint {
	return b.checkpoints
}

