package blockchain

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"container/list"
	"fmt"
	"github.com/btcsuite/btcutil"
	"sync"
	"time"
)

//blocklocator is used to help locate a specific block.the algorithm for
//buliding the block locator is to used the hashes in reverse order until
//the genesis block is reached.in order to keep the list of locator hashes
//to a reasonable number of entries,first the most recent previous 12 block
//hashes are added ,then the step is doubled each loop interation to exponenttially
//decrease the number of hashes as a function of the distance from the block
//being located

// For example, assume a block chain with a side chain as depicted below:
// 	genesis -> 1 -> 2 -> ... -> 15 -> 16  -> 17  -> 18
// 	                              \-> 16a -> 17a
//
// The block locator for block 17a would be the hashes of blocks:
// [17a 16a 15 14 13 12 11 10 9 8 7 6 4 genesis]
type BlockLocator []*chainhash.Hash

//orphanblock representes a block that we do not yet have the parent for.
//it is a normal block plus an expiraction time to prevent caching the orphan
//forever.
type orphanBlock struct {
	block      *btcutil.Block
	expiration time.Time
}

//beststate houses information about the current best block and other
//info related to the state fo the main chain as it exists from the
//point of view of the current best block.

//the bsetsnapshot method can be used to obtain access to this information
//in a concurrent safe manner and the data will not be changed out formtunder
//the caller when chain state changes occur as the fucntion name implies.
//however the returned snapshot must be treated as immutable since it si
//shared by all caller.

type BestState struct {
	Hash        chainhash.Hash // 块的哈希值
	Height      int32          // 块的高度
	Bits        uint32         // 块的难度
	BlockSize   uint64         //块的大小
	BlockWeight uint64         // 块的重量
	NumTxns     uint64         // 块中的txn的数组
	TotalTxns   uint64         //链中Txn的总数
	MedianTime  time.Time      // 根据calcpastmediantime确定的中间时间
}

//newbeststate returns a new best states instance for the given parameters.
func newBestState(node *blockNode, blockSize, blockWeight, numTxns, totalTxns uint64,
	medianTime time.Time) *BestState {
	return &BestState{
		Hash:        node.hash,
		Height:      node.height,
		Bits:        node.bits,
		BlockSize:   blockSize,
		BlockWeight: blockWeight,
		NumTxns:     numTxns,
		TotalTxns:   totalTxns,
		MedianTime:  medianTime,
	}
}

// BlockChain provides functions for working with the bitcoin block chain.
// It includes functionality such as rejecting duplicate blocks, ensuring blocks
// follow all rules, orphan handling, checkpoint handling, and best chain
// selection with reorganization.
type BlockChain struct {
	// The following fields are set when the instance is created and can't
	// be changed afterwards, so there is no need to protect them with a
	// separate mutex.
	checkpoints         []chaincfg.Checkpoint
	checkpointsByHeight map[int32]*chaincfg.Checkpoint
	db                  database.DB
	chainParams         *chaincfg.Params
	timeSource          MedianTimeSource
	sigCache            *txscript.SigCache
	indexManager        IndexManager
	hashCache           *txscript.HashCache

	// The following fields are calculated based upon the provided chain
	// parameters.  They are also set when the instance is created and
	// can't be changed afterwards, so there is no need to protect them with
	// a separate mutex.
	minRetargetTimespan int64 // target timespan / adjustment factor
	maxRetargetTimespan int64 // target timespan * adjustment factor
	blocksPerRetarget   int32 // target timespan / target time per block

	// chainLock protects concurrent access to the vast majority of the
	// fields in this struct below this point.
	chainLock sync.RWMutex

	// These fields are related to the memory block index.  They both have
	// their own locks, however they are often also protected by the chain
	// lock to help prevent logic races when blocks are being processed.
	//
	// index houses the entire block index in memory.  The block index is
	// a tree-shaped structure.
	//
	// bestChain tracks the current active chain by making use of an
	// efficient chain view into the block index.
	index     *blockIndex
	bestChain *chainView

	// These fields are related to handling of orphan blocks.  They are
	// protected by a combination of the chain lock and the orphan lock.
	orphanLock   sync.RWMutex
	orphans      map[chainhash.Hash]*orphanBlock
	prevOrphans  map[chainhash.Hash][]*orphanBlock
	oldestOrphan *orphanBlock

	// These fields are related to checkpoint handling.  They are protected
	// by the chain lock.
	nextCheckpoint *chaincfg.Checkpoint
	checkpointNode *blockNode

	// The state is used as a fairly efficient way to cache information
	// about the current best chain state that is returned to callers when
	// requested.  It operates on the principle of MVCC such that any time a
	// new block becomes the best block, the state pointer is replaced with
	// a new struct and the old state is left untouched.  In this way,
	// multiple callers can be pointing to different best chain states.
	// This is acceptable for most callers because the state is only being
	// queried at a specific point in time.
	//
	// In addition, some of the fields are stored in the database so the
	// chain state can be quickly reconstructed on load.
	stateLock     sync.RWMutex
	stateSnapshot *BestState

	// The following caches are used to efficiently keep track of the
	// current deployment threshold state of each rule change deployment.
	//
	// This information is stored in the database so it can be quickly
	// reconstructed on load.
	//
	// warningCaches caches the current deployment threshold state for blocks
	// in each of the **possible** deployments.  This is used in order to
	// detect when new unrecognized rule changes are being voted on and/or
	// have been activated such as will be the case when older versions of
	// the software are being used
	//
	// deploymentCaches caches the current deployment threshold state for
	// blocks in each of the actively defined deployments.
	warningCaches    []thresholdStateCache
	deploymentCaches []thresholdStateCache

	// The following fields are used to determine if certain warnings have
	// already been shown.
	//
	// unknownRulesWarned refers to warnings due to unknown rules being
	// activated.
	//
	// unknownVersionsWarned refers to warnings due to unknown versions
	// being mined.
	unknownRulesWarned    bool
	unknownVersionsWarned bool

	// The notifications field stores a slice of callbacks to be executed on
	// certain blockchain events.
	notificationsLock sync.RWMutex
	notifications     []NotificationCallback
}

//haveblock returns whether or not the chain instance has the block represented
//by the passed hash. this include checking the various places a block can
//be likely part of the main chain. on a side chain .or in the orphan pool.
func (b *BlockChain) HaveBlock(hash *chainhash.Hash) (bool, error) {
	exists, err := b.blockExists(hash)
	if err != nil {
		return false, err
	}

	return exists || b.IsKnownOrphan(hash), nil
}

// IsKnownOrphan returns whether the passed hash is currently a known orphan.
// Keep in mind that only a limited number of orphans are held onto for a
// limited amount of time, so this function must not be used as an absolute
// way to test if a block is an orphan block.  A full block (as opposed to just
// its hash) must be passed to ProcessBlock for that purpose.  However, calling
// ProcessBlock with an orphan that already exists results in an error, so this
// function provides a mechanism for a caller to intelligently detect *recent*
// duplicate orphans and react accordingly.

//this function is safe for concurrent access.
func (b *BlockChain) IsKnownOrphan(hash *chainhash.Hash) bool {
	//protect concurrent access.using a raad lock only so multiple
	//readers can query without blcoking each other.
	b.orphansLock.RLock()
	_, exists := b.orphans[*hash]
	b.orphanLock.RUnlock()

	return exists
}

//getorphanroot returns the head of the chain for the provided hash from the
//map of orphan blocks.
//this function is safe for concurrent accesss.
func (b *BlockChain) GetOrphanRoot(hash *chainhash.Hash) *chainhash.Hash {
	//protect concrurent access. using a read lock only so multiple
	//readrs can query without blocking each other.
	b.orphanLock.RLock()
	defer b.orphanLock.RUnlock()

	//keeping loopping while the parent of each orphaned block is known and id
	//an orphan iteself
	orphanRoot := hash
	prevHash := hash
	for {
		orphan, exists := b.orphans[*prevHash]
		if !exists {
			break
		}

		orphanRoot = prevHash
		prevHash = &orphan.block.MsgBlock().Header.PrevBlock
	}
	return orphanRoot

}

//removeorphanblock removes the passed orphan block from the orphan pool
//and previous orphan index.
func (b *BlockChain) removeOrphanBlock(orphan *orphanBlock) {
	//protect concurrent access
	b.orphanLock.Lock()
	defer b.orphanLock.Unlock()

	//remove the orphan block from the orphan pool
	orphanHash := orphan.block.Hash()
	delete(b.orphans, *orphanHash)

	//remove the reference form the previous orphan index too. an indexing
	//for loop is intentionally used over a range here as range does not
	//reevaluate the slice on each interation nor does it adjust the index
	//for the modified slice.
	prevHash := &orphan.block.MsgBlock().Header.PrevBlock
	orphans := b.prevOrphans[*prevHash]
	for i := 0; i < len(orphans); i++ {
		hash := orphans[i].block.Hash()
		if hash.IsEqual(orphanHash) {
			copy(orphans[i:], orphans[i+1:])
			orphans[len(orphans)-1] = nil
			orphans = orphans[:len(orphans)-1]
			i--
		}
	}

	b.prevOrphans[*prevHash] = orphans

	//remove the map entry altogether if there are no longer any orphans
	//which depned on the parent hash.
	if len(b.prevOrphans[*prevHash]) == 0 {
		delete(b.prevOrphans, *prevHash)
	}

}

//addorphanblock adds the passed block (which is already determinded to be
//an orphan prior calling this function)to the orphan pool. it lazily cleans
//up any expired blocks as a separate cleanup poller do not need to be run.
//it also imposes a maximum limit on the number of outstnading orphan
//blocks and will the oldest received orphans block if the limit is excessecd.
func (b *BlockChain) addOrphanBlock(block *btcutil.Block) {
	//remove expired orphan blocks.
	for _, oBlock := range b.orphans {
		if time.Now().After(oBlock.expiration) {
			b.removeOrphanBlock(oBlock)
			continue
		}

		//update the oldeset orphan block pointer so it can be discarded
		//in case the orphan pool fills up.
		if b.oldestOrphan == nil || oBlock.expiration.Before(b.oldestOrphan.expiration) {
			b.oldestOrphan = oBlock
		}

	}

	//limit orphan blocks to prevent memory exhaustion.
	if len(b.orphans)+1 > maxOrphanBlocks {
		//remove the oldest orphan to make room for the new one.
		b.removeOrphanBlock(b.oldestOrphan)
		b.oldestOrphan = nil
	}

	// Protect concurrent access.  This is intentionally done here instead
	// of near the top since removeOrphanBlock does its own locking and
	// the range iterator is not invalidated by removing map entries.
	b.orphanLock.Lock()
	defer b.orphanLock.Unlock()

	//insert the block into the orphan map with an exporation time
	//1 hour from now.
	expiration := time.Now().Add(time.Hour)
	oBlock := &orphanBlock{
		block:      block,
		expiration: expiration,
	}
	b.orphans[*block.Hash()] = oBlock

	//add to previous hash lookup index for faster dependency lookups.
	prevHash := &block.MsgBlock().Header.PrevBlock
	b.prevOrphans[*prevHash] = append(b.prevOrphans[*prevHash], oBlock)

}

// SequenceLock represents the converted relative lock-time in seconds, and
// absolute block-height for a transaction input's relative lock-times.
// According to SequenceLock, after the referenced input has been confirmed
// within a block, a transaction spending that input can be included into a
// block either after 'seconds' (according to past median time), or once the
// 'BlockHeight' has been reached.
type SequenceLock struct {
	Seconds     int64
	BlockHeight int32
}

// CalcSequenceLock computes a relative lock-time SequenceLock for the passed
// transaction using the passed UtxoViewpoint to obtain the past median time
// for blocks in which the referenced inputs of the transactions were included
// within. The generated SequenceLock lock can be used in conjunction with a
// block height, and adjusted median block time to determine if all the inputs
// referenced within a transaction have reached sufficient maturity allowing
// the candidate transaction to be included in a block.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcSequenceLock(tx *btcutil.Tx, utxoView *UtxoViewpoint, mempool bool) (*SequenceLock, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	return b.calcSequenceLock(b.bestChain.Tip(), tx, utxoView, mempool)
}

// calcSequenceLock computes the relative lock-times for the passed
// transaction. See the exported version, CalcSequenceLock for further details.
//
// This function MUST be called with the chain state lock held (for writes).

func (b *BlockChain) calcSequenceLock(node *blockNode, tx *btcutil.Tx, utxoView *UtxoViewpoint, mempool bool) (*SequenceLock, error) {

	//a value of -1 for each relative lock type represents a relative time lock value that will allow a transaction
	//to be included in a block an any given height or time . this value is returned as the relative lock time
	//in the case that bip68 is disable or has not yet been actived .
	SequenceLock := &SequenceLock{Seconds: -1, BlockHeight: -1}

	//the sequence locks semantics are always active for transactions within the mempool
	csvSoftforkActive := mempool

	//if we`re perfoming blcok validation .then we need to query the bip9 state.
	if !csvSoftforkActive {
		//obtain the lastest bip9 version bits state for the
		//csv_package soft-fork deploment. the adherence of squeence
		//locks depneds on the current soft-fork state.
		csvState, err := b.deploymentState(node.parent, chaincfg.DeploymentCSV)
		if err != nil {
			return nil, err
		}

		csvSoftforkActive = csvState == ThresholdActive
	}

	mTx := tx.MsgTx()
	SequenceLockActive := mTx.Version >= 2 && csvSoftforkActive
	if !SequenceLockActive || IsCoinBase(tx) {
		return SequenceLock, nil
	}

	//grab the next height form the pov of the passed blocknode to use for
	//inputs present in the mempool.
	nextHeigt := node.height + 1

	for txInIndex, txIn := range mTx.TxIn {
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil {
			str := fmt.Sprintf("output %v referenced from "+
				"transaction %s :%d eight does not exist or "+
				"has already been spent", txIn.PreviousOutPoint, tx.Hash(), txInIndex)
			return SequenceLock, ruleError(ErrMissingTxOut, str)
		}

		//if the input height is set to te mempool height ,then we assume the transaction
		//makes ti into the nex block when evealuating its sequence blocks.

		inputHeight := utxo.BlockHeight()
		if inputHeight == 0x7fffffff {
			inputHeight = nextHeigt
		}

		//given a sequence number,we apply the relative time lock mask in order
		//to obtain the time lock delta required before this input can be spnet.
		sequenceNum := tx.In.Sequence
		relativeLock := int64(sequenceNum & wire.SequenceLockTimeMask)

		switch {

		//relative time locks are disabled for this input .so we can
		//skip any further calculation .
		case sequenceNum&wire.SequenceLockTimeDisabled == wire.SequenceLockTimeDisabled:
			continue
		case sequenceNum&wire.SequenceLockTimeIsSeconds == wire.SequenceLockTimeIsSeconds:
			//this input requires a relative time lock expressed in seconds before it can
			//be spent. therefore .we neeed to query for the block prior to the one in
			//which this input was included within so we can compute the past median time or
			//the block prior to the one which included this referenced output.
			prevInputHeight := inputHeight - 1
			if prevInputHeight < 0 {
				prevInputHeight = 0
			}

			blockNode := node.Ancestor(prevInputHeight)
			medianTime := blockNode.CalcPastMedianTime()

			//time based relative time-locks as difined by bip 68 have a time
			//granularity of relativelockseconds so we shift left by this amount
			//to convert to the proper relative time-lock .we also subtract one from
			//the relative lock to maintain the original locktime semantics.
			timeLockSeconds := (relativeLock << wire.SequenceLockTimeGranularity) - 1
			timeLock := medianTime.Unix() + timeLockSeconds
			if timeLock > SequenceLock.Seconds {
				SequenceLock.Seconds = timeLock
			}

		default:
			//the rlative lock-time for this input is expressed in blocks so we
			//cauculate the relative offset from the input`s height as its converted absolute
			//lock-time.we subtract one from the relative lock in order to maintain the originnal
			//locktime semantics.
			blockHeight := inputHeight + int32(relativeLock-1)
			if blockHeight > SequenceLock.BlockHeight {
				SequenceLock.BlockHeight = blockHeight
			}

		}
	}
	return SequenceLock, nil
}

// LockTimeToSequence converts the passed relative locktime to a sequence
// number in accordance to BIP-68.
// See: https://github.com/bitcoin/bips/blob/master/bip-0068.mediawiki
//  * (Compatibility)
func LockTimeToSequence(isSeconds bool, locktime uint32) uint32 {
	// If we're expressing the relative lock time in blocks, then the
	// corresponding sequence number is simply the desired input age.
	if !isSeconds {
		return locktime
	}

	// Set the 22nd bit which indicates the lock time is in seconds, then
	// shift the locktime over by 9 since the time granularity is in
	// 512-second intervals (2^9). This results in a max lock-time of
	// 33,553,920 seconds, or 1.1 years.
	return wire.SequenceLockTimeIsSeconds |
		locktime>>wire.SequenceLockTimeGranularity
}

//get reorgannizenodes finds the fork point between the main chain adn
//the passed node and returns a list of block nodes that would need to be detached
//from the main chain and a list of block nodes that would need to be
//attached to the point(which will be the end of the main chain after detaching tthe
//returned list of block nodes )in order to reorganize the chain such atht the
//passed node is the new end of the main chain. the lists will be empty if the
//passed node is not on a side chain.
//this function may modify node statuses in the block index without fulshing

func (b *BlockChain) getReorganizeNodes(node *blockNode) (*list.List ,*list.List) {
	attachNodes := list.New()
	detachNodes := list.New()

	if b.index.NodeStatus(node.parent).KnownInvalid(){
		b.index.SetStatusFlags(node,statusInvalidAncestor)
		return detachNodes,attachNodes
	}




}



















