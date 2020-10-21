package blockchain

import "BtcoinProject/chaincfg/chainhash"

//behaviorflag is a bitmask difining tweaks to the normal behavior when performing
//chain processing and consesus rules checks.
type BehaviorFlags uint32

const (
	//bffastadd may be set to indicate that several checks can be avioded for the block since it is
	//already known to fit into the chain due to already proving it corretcint links
	//into the chain up to a known checkpoint .this is primaryly used for headers-ifrst mode.
	BFFastAdd BehaviorFlags = 1 << iota

	//bfnoppowcheck may be set to indicate the proof of work check which ensure
	//a block hashed to a value less than the required target will not be performed.
	BFNoPoWCheck

	//bfnone is a convenience value to specifically indicate no flags
	BFNone BehaviorFlags = 0
)

//blockexists determains whether a block with the given hash exists either in the main /
//chain or any side chains .

//this function is safe for concurrent access.
func (b *BlockChain) blockExists(hash *chainhash.Hash) (bool, error) {
	//check block index first (cound be main chain or side chain blocks)
	if b.index.HaveBlock(hash) {
		return true, nil
	}

	//check in the database.
	var exists bool
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		exists, err = dbTx.HasBlock(hash)
		if err != nil || !exists {
			return err
		}

		//ingore side chain blocks in the database.this is necessary because
		//there is not currently any recode of the associated block index data
		//such as ites block height .so it is not yet possible to effiently load
		//the block and do anything useful with it .
		//ultimately the entire block index should be serialized instead of only the curent
		//main so it can be consulted directly.
		_, err = dbFetchHeightByHash(dbTx, hash)
		if isNotInMainChainErr(err) {
			exists = false
			return nil
		}

		return err

	})

	return exists, err
}


// processOrphans determines if there are any orphans which depend on the passed
// block hash (they are no longer orphans if true) and potentially accepts them.
// It repeats the process for the newly accepted blocks (to detect further
// orphans which may no longer be orphans) until there are no more.
//
// The flags do not modify the behavior of this function directly, however they
// are needed to pass along to maybeAcceptBlock.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) processOrphans(hash *chainhash.Hash, flags BehaviorFlags) error {
	// Start with processing at least the passed hash.  Leave a little room
	// for additional orphan blocks that need to be processed without
	// needing to grow the array in the common case.
	processHashes := make([]*chainhash.Hash, 0, 10)
	processHashes = append(processHashes, hash)
	for len(processHashes) > 0 {
		// Pop the first hash to process from the slice.
		processHash := processHashes[0]
		processHashes[0] = nil // Prevent GC leak.
		processHashes = processHashes[1:]

		// Look up all orphans that are parented by the block we just
		// accepted.  This will typically only be one, but it could
		// be multiple if multiple blocks are mined and broadcast
		// around the same time.  The one with the most proof of work
		// will eventually win out.  An indexing for loop is
		// intentionally used over a range here as range does not
		// reevaluate the slice on each iteration nor does it adjust the
		// index for the modified slice.
		for i := 0; i < len(b.prevOrphans[*processHash]); i++ {
			orphan := b.prevOrphans[*processHash][i]
			if orphan == nil {
				log.Warnf("Found a nil entry at index %d in the "+
					"orphan dependency list for block %v", i,
					processHash)
				continue
			}

			// Remove the orphan from the orphan pool.
			orphanHash := orphan.block.Hash()
			b.removeOrphanBlock(orphan)
			i--

			// Potentially accept the block into the block chain.
			_, err := b.maybeAcceptBlock(orphan.block, flags)
			if err != nil {
				return err
			}

			// Add this block to the list of blocks to process so
			// any orphan blocks that depend on this block are
			// handled too.
			processHashes = append(processHashes, orphanHash)
		}
	}
	return nil
}





















