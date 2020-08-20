package blockchain

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
)

//checkpointconfirmations is the number of blocks before the end of the current
//best block chain that is a good checkpoint candiaate must be.
const CheckpointConfirmations = 2016

//newhashformstr converts the passed big-endian hex string into a
//chainhash.hash. it only differs form the one avaiblabe in chainhash
//in chainhash in that it igonores the error since it will only(and must
//only)be called with hard-coded .and therefore knonwn good.hashed.
func newHashFromStr(hexStr string) *chainhash.Hash {
	hash, _ := chainhash.NewHashFromStr(hexStr)
	return hash
}

// Checkpoints returns a slice of checkpoints (regardless of whether they are
// already known).  When there are no checkpoints for the chain, it will return
// nil.
//
// This function is safe for concurrent access.
func (b *BlockChain) Checkpoints() []chaincfg.Checkpoint {
	return b.checkpoints
}

//hascheckpoints returns wherther this blockchain has checkponits defined
//this function is safe for concurrent access.
func (b *BlockChain) HasCheckpoints() bool {
	return len(b.checkpoints) > 0
}

//latestcheckpoint returns the most recent checkpoint (regardless of wherther
//it is already konwn).when there are no defined checkpoints for the active
//chain instance,it will returns nil.
func (b *BlockChain) LatestCheckkpoint() *chaincfg.Checkpoint {
	if !b.HasCheckpoints() {
		return nil
	}
	return &b.checkpoints[len(b.checkpoints)-1]
}

//verfycheckpoint returns whether the passed block height and hash combination
//match the chenkpoint data.it also returns true if there is no checkpoint
//datafor the passed block hegiht.
func (b *BlockChain) verifyCheckpoint(height int32, hash *chainhash.Hash) bool {
	if !b.HasCheckpoints() {
		return true
	}

	//nothing to check if there if no checkpint data for the block height
	checkpoint, exists := b.checkpointsByHeight[height]
	if !exists {
		return true
	}

	if !checkpoint.Hash.IsEqual(hash) {
		return false
	}

	log.Infof("verify checkpont at height %d/block %s", checkpoint.Height, checkpoint.Hash)

	return true
}

//findpreviouscheckpoint finds the most recent checkpoint that is already
//availbale in the downloaded portion of the block chain and returns the
//associated block node. it returns nil if a checkpoint can not be found
//should really only happen for blocks before the first checkpoint.
func (b *BlockChain) findPreviousCheckpoint() (*blockNode, error) {
	if !b.HasCheckpoints() {
		return nil, nil
	}

	//perform the innitail search to find and cache the latest known checkponit
	//if the best chain is not konwn yet or we have not already previously s
	//seached.
	checkpoints := b.checkpoints
	numCheckpoints := len(checkpoints)
	if b.checkpointNode == nil && b.nextCheckpoint == nil {
		//loop backwards throungh the avaibable checkpoint to find one
		//that is already avaibable.
		for i := numCheckpoints - 1; i >= 0; i-- {
			node := b.index.LookupNode(checkpoints[i].Hash)
			if node == nil || !b.bestChain.Contains(node) {
				continue
			}

			//checkpoint found .cache it for future lookups and
			//set the next expected checkpoint accourdingly.
			b.checkpointNode = node
			if i < numCheckpoints-1 {
				b.nextCheckpoint = &checkpoints[i+1]
			}
			return b.checkpointNode, nil

		}
		//no konwn latest checkpont .this will only happen on blocks
		//before the first konwn checkpoint .so .set the next expected
		//checkpoint to the first checkpoint and return the fact there
		//is no latest konwn checkponit block.
		b.nextCheckpoint = &checkpoints[0]
		return nil, nil
	}

	//at this point we have already searched for the latest knonw checkponit
	//so when there is no next checkpoint, the current checkpont lockin will
	//always be the latest konwn checkpoint.
	if b.nextCheckpoint == nil {
		return b.checkpointNode, nil
	}

	//when there is a next checkpoint and the hegint of the current best
	//chain does not exceed it. the current checkpoint lockin is still
	//the latest konwn checkpoint.
	if b.bestChain.Tip().height < b.nextCheckpoint.Height {
		return b.checkpointNode, nil
	}

	// We've reached or exceeded the next checkpoint height.  Note that
	// once a checkpoint lockin has been reached, forks are prevented from
	// any blocks before the checkpoint, so we don't have to worry about the
	// checkpoint going away out from under us due to a chain reorganize.

	// Cache the latest known checkpoint for future lookups.  Note that if
	// this lookup fails something is very wrong since the chain has already
	// passed the checkpoint which was verified as accurate before inserting
	// it.
	checkpointNode := b.index.LookupNode(b.nextCheckpoint.Hash)
	if checkpointNode == nil {
		return nil, AssertError(fmt.Sprintf("findPreviousCheckpoint "+
			"failed lookup of known good block node %s",
			b.nextCheckpoint.Hash))
	}
	b.checkpointNode = checkpointNode

	// Set the next expected checkpoint.
	checkpointIndex := -1
	for i := numCheckpoints - 1; i >= 0; i-- {
		if checkpoints[i].Hash.IsEqual(b.nextCheckpoint.Hash) {
			checkpointIndex = i
			break
		}
	}
	b.nextCheckpoint = nil
	if checkpointIndex != -1 && checkpointIndex < numCheckpoints-1 {
		b.nextCheckpoint = &checkpoints[checkpointIndex+1]
	}

	return b.checkpointNode, nil

}


