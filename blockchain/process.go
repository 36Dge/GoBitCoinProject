package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"github.com/btcsuite/btcutil"
	"time"
)

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

//processoorphans determines if there any orphans which depned on the passed
//block hash(they are no longer orphans if true)and potentially accepts them.
//it repeats the process for the newly acctped blocks(to detect further orpahsn
//which may no longer be orphans)until there are no more.

//the flags do not modify the behavior of this function diretcly however they
//are needed to pass along to maybeacceptblock.
//this function must be called with the chain state lock held(for writes).

func (b *BlockChain) processOrphans(hash *chainhash.Hash, flags BehaviorFlags) error {

	//start with processing at least the passed hash. leave a little room
	//for additional orphans blocks that need to be processed without needing
	//to grow the array in the common case.
	processHashes := make([]*chainhash.Hash, 0, 10)
	processHashes = append(processHashes, hash)

	for len(processHashes) > 0 {
		//pop the first hash to process form the slice
		processHash := processHashes[0]
		processHashes[0] = nil //provent CG leak.
		processHashes = processHashes[1:]

		//look up all orphans that are parented by the block we just
		//accepted. this will typically only be one .but it could be multiple
		//if multiple blocks are mined and broadcast around the same time.
		//the one win out. an indexing for loop is intentionally used over a
		//range here as range does not reevalaute the slice on each iteration
		//nor does it adjust the index for the modified slice.
		for i := 0; i < len(b.prevOrphans[*processHash]); i++ {
			orphan := b.prevOrphans[*processHash][i]
			if orphan == nil {
				log.Warnf("found a nil entry at index %d in the "+
					"orphan dependency list for block %v", i, processHash)
				continue
			}

			//remove the orphan from the orphan pool
			orphanHash := orphan.block.Hash()
			b.removeOrphanBlock(orphan)
			i--

			//protentially accept the block into the block chain.
			_, err := b.maybeAcceptBlock(orphan.block, flags)
			if err != nil {
				return err
			}

			//add this block to the list of blocks to process so
			//any orphan blocks that depend on this block are handled too.
			processHashes = append(processHashes, orphanHash)

		}

	}

	return nil
}

//processblock is the main workhorse for handling inseration of new blocks into
//the block chain it inculudes functionality such as rejecting duplicae
//blocks ensuring blocks follow all rules orphan handling .and insertion into
//the block chain along with best chain selection and reorganization.

//when no errors occured during processing the first return value indicates
//whether or not the block is on the main chain and the second indicates whether
//or not the block is an orphan
//this function is safe for concurrent access.
func (b *BlockChain) ProcessBlock(block *btcutil.Block, flags BehaviorFlags) (bool, bool, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	fastAdd := flags&BFFastAdd == BFFastAdd

	blockHash := block.Hash()
	log.Tracef("Processing block %v", blockHash)

	//the block must not already exist in the main chain or side chains.
	exists, err := b.blockExists(blockHash)
	if err != nil {
		return false, false, err
	}
	if exists {
		str := fmt.Sprintf("already have block %v", blockHash)
		return false, false, ruleError(ErrDuplicateBlock, str)
	}

	//the block must not already exist as an orphan
	if _, exists := b.orphans[*blockHash]; exists {
		str := fmt.Sprintf("already have block(orphan)%v", blockHash)
		return false, false, ruleError(ErrDuplicateBlock, str)
	}

	//perform proliminary sanity checks on the block and its transaction
	err = checkBlockSanity(block, b.chainParams.PowLimit, b.timeSource, flags)
	if err != nil {
		return false, false, err
	}

	//find the prvious checkpoint and perform some additional checks based
	//on the checkpoint.this provides a few nice properties such as preventing
	//old side chain blocks before the last checkpoint.rejecting easy to mine
	//but otherwise bogus.blocks that could be used to eat memory and ensuring
	//expected(versus claimed)proof of work requirements since the previous checkpoint are
	//met.
	blockHeader := &block.MsgBlock().Header
	checkpointNode, err := b.findPreviousCheckpoint()
	if err != nil {
		return false, false, err
	}
	if checkpointNode != nil {
		//ensure the block timestamp is after the checkpoint timestamp
		checkpointTime := time.Unix(checkpointNode.timestamp, 0)
		if blockHeader.Timestamp.Before(checkpointTime) {
			str := fmt.Sprintf("block %v has timestamp %v before "+
				"last checkpoint timestamp %v", blockHash, blockHeader.Timestamp, checkpointTime)
			return false, false, ruleError(ErrCheckpointTimeTooOld, str)
		}

		if !fastAdd {
			//even though the checks prior to now have already ensured the
			//proof of work exceeds the claimed amount ,the claimed amount
			//is a field in the block header which could be forged ,this checks
			//ensures the proof of work is at least the minimum expetced based ton
			//elapsed time since the last checkpoint and maximum adjust allowded byte retreget rules.

			duration := blockHeader.Timestamp.Sub(checkpointTime)
			requiredTarget := CompactToBig(b.calcEasiestDifficulty(checkpointNode.bits, duration))

			currentTarget := CompactToBig(blockHeader.Bits)
			if currentTarget.Cmp(requiredTarget) > 0 {
				str := fmt.Sprintf("block target difficulty of %064x"+
					"checkpoint", currentTarget)
				return false, false, ruleError(ErrDifficultyTooLow, str)
			}

		}

	}

	//handle orphan blocks.
	prevHash := &blockHeader.PrevBlock
	prevHashExists, err := b.blockExists(prevHash)
	if err != nil {
		return false, false, err
	}

	if !prevHashExists {
		log.Infof("adding orphan block %v with parent %v", blockHash, prevHash)
		b.addOrphanBlock(block)
		return false, true, nil
	}

	//the block has passed all context independent checks and appears sane
	//enough to potentially accept it into the block chain.
	isMainChain, err := b.maybeAcceptBlock(block, flags)
	if err != nil {
		return false, false, err
	}

	//accept any orphan blocks that depend on this block(they are no loger orphans )and repeat for
	//those accepted blocks until there are no more.
	err = b.processOrphans(blockHash, flags)
	if err != nil {
		return false, false, err
	}

	log.Debugf("accept block %v", blockHash)

	return isMainChain, false, nil

}

//over
