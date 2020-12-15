package blockchain

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
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

func (b *BlockChain) getReorganizeNodes(node *blockNode) (*list.List, *list.List) {
	attachNodes := list.New()
	detachNodes := list.New()

	if b.index.NodeStatus(node.parent).KnownInvalid() {
		b.index.SetStatusFlags(node, statusInvalidAncestor)
		return detachNodes, attachNodes
	}

	//find the fork point (if any) adding each block to the list of nodes
	//to attach to the main tree. push them onto the list in reverse order
	//so they are attached in the appropriate order when iterating the list
	//later.
	forkNode := b.bestChain.FindFork(node)
	invalidChain := false
	for n := node; n != nil && n != forkNode; n = n.parent {
		if b.index.NodeStatus(n).KnownInvalid() {
			invalidChain = true
			break
		}

		attachNodes.PushFront(n)
	}

	// if any of the node is ancestors are invalid .unwind attachNodes ,marking
	// each on as invalid for further reference
	if invalidChain {
		var next *list.Element
		for e := attachNodes.Front(); e != nil; e = next {
			next = e.Next()
			n := attachNodes.Remove(e).(*blockNode)
			b.index.SetStatusFlags(n, statusInvalidAncestor)

		}
		return detachNodes, attachNodes
	}

	//start from the end of main chain and work backwards until the common
	//ancestor adding each block to the list of the nodes to detach from
	//the main chain.

	for n := b.bestChain.tip(); n != nil && n != forkNode; n = n.parent {
		detachNodes.PushBack(n)
	}

	return detachNodes, attachNodes

}

//connectblock handles conneting the passed node/block to the end of the mian
//best chain.
//this passed utxo view must have all referenced txos the block spends marked
//as spent and all of the new txos the block creates added to it .in addition .
//the passed stxos slice must be populated with all of the information for the
//spents txos. this approach is used because the connection validation that
//must happen prior to calling this function requires the same details .so
//it would be inefficient to repeat it.
func (b *BlockChain) connectBlock(node *blockNode, block *btcutil.Block,
	view *UtxoViewpoint, stxos []SpentTxOut) error {

	//make sure it is extending the end of the best chain.
	prevHash := &block.MsgBlock().Header.PrevBlock
	if !prevHash.IsEqual(&b.bestChain.Tip().hash) {
		return AssertError("connectBlock must be called with a block " +
			"that extends the main chain.")
	}

	//sanity check the correct number of stxos are provided.
	if len(stxos) != countSpentOutputs(block) {
		return AssertError("connectBlock called with inconsistent" +
			"spent transaction out information")
	}

	//no warning about unkonwn rules or versions until the chain is current
	if b.isCurrent() {
		//warn if any unknown new rules are either about to activate or
		//have already been activated.
		if err := b.warnUnkonwnRuleActivations(node); err != nil {
			return err
		}

		//warn if a high enough percentage of the last blocks have unexpected verions.
		if err := b.warnUnkonwnRuleActivations(node); err != nil {
			return err
		}

	}

	//write any block status changes to DB before updating best state.
	err := b.index.flushToDB()
	if err != nil {
		return err
	}

	//generate a new best state snapshot will be used to update the database
	//and later memory if all database updates are successful.
	b.stateLock.RLock()
	curTotalTxns := b.stateSnapshot.TotalTxns
	b.stateLock.RUnlock()
	numTxns := uint64(len(block.MsgBlock().Transactions))
	blockSize := uint64(block.MsgBlock().SerializeSize())
	blockWeight := uint64(GetBlockWeight(block))
	state := newBestState(node, blockSize, blockWeight, numTxns, curTotalTxns+numTxns,
		node.CalcPastMedianTime())
	//atomically insert into the database
	err = b.db.Update(func(dbTx database.Tx) error {
		//update best block state.
		err := dbPutBestState(dbTx, state, node.workSum)
		if err != nil {
			return err
		}

		//add the block hash and height to block index which tracks
		//the main chain,
		err = dbPutBlockIndex(dbTx, block.Hash(), node.height)
		if err != nil {
			return err
		}

		//update the utxo set using the state of the utxo view .this
		//entails removing all of the utxos spent and adding the new
		//ones created by the block.
		err = dbPutUtxoView(dbTx, view)
		if err != nil {
			return err
		}

		//update the transaction spend journal by adding a record for
		//the block that contains all txos spent by it.
		err = dbPutSpendJournalEntry(dbTx, block.Hash(), stxos)
		if err != nil {
			return err
		}

		//allow the index manager to call each of the curretly active
		//optional index with the block being connected so they can
		//update themselves accordingly.
		if b.indexManager != nil {
			err := b.indexManager.ConnectBlock(dbTx, block, stxos)
			if err != nil {
				return err
			}
		}

		return nil

	})

	if err != nil {
		return err
	}

	//prune fully spent entries and mark all entries in the view unmodifyed
	//now that the modification have been committed to the database
	view.commit()

	//this node is now the end of the best chain.
	b.bestChain.setTip(node)

	//update the state for the best block.notice how this replaces the
	//entries struct instead of updating the existing one.this effectively
	// allows the old version to act as a snapshot which callers can use
	// freely without needing to hold a lock for the duration.  See the
	// comments on the state variable for more details.
	b.stateLock.Lock()
	b.stateSnapshot = state
	b.stateLock.Unlock()

	//notify the caller that the block was connected to the main chain.
	//the caller would typcically want to react with actions such as
	//updating wallets.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockConnected, block)
	b.chainLock.Lock()

	return nil

}

//disconnectblock handles disconnecting the passed  node /block from the
//the end of the mian (best)chain.
//this function must be called with the chain state lock held (for wriets).
func (b *BlockChain) disconnectBlock(node *blockNode, block *btcutil.Block, view *UtxoViewpoint) error {

	//make sure the node being disconnected is the ent of the best chain.
	if !node.hash.IsEqual(&b.bestChain.Tip().hash) {
		return AssertError("disconectBlock must be called with the " +
			"block at the end of the main chain")
	}

	//load the previous block since some details for it are needed below.
	prevNode := node.parent
	var prevBlock *btcutil.Block
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		prevBlock, err = dbFetchBlockByNode(dbTx, prevNode)
		return err
	})
	if err != nil {
		return err
	}

	//write any block status changes to DB before updating best state.
	err = b.index.flushToDB()
	if err != nil {
		return err
	}

	//generate a new best state snapshot that will be used to update the
	//database and later memory if all database updates are successful.

	b.stateLock.RLock()
	curTotalTxns := b.stateSnapshot.TotalTxns
	b.stateLock.RUnlock()
	numTxns := uint64(len(prevBlock.MsgBlock().Transactions))
	blockSize := uint64(prevBlock.MsgBlock().SerializeSize())
	blockWeight := uint64(GetBlockWeight(prevBlock))
	newTotalTxns := curTotalTxns - uint64(len(block.MsgBlock().Transactions))
	state := newBestState(prevNode, blockSize, blockWeight, numTxns, newTotalTxns, prevNode.CalcPastMedianTime())

	err = b.db.Update(func(dbTx database.Tx) error {
		//update best block state.
		err := dbPutBestState(dbTx, state, node.workSum)
		if err != nil {
			return err
		}

		//remove the block hash and height form the block index which track
		//the main chain.
		err = dbRemoveBlockIndex(dbTx, block.Hash(), node.height)
		if err != nil {
			return err
		}

		//update the utxos set using the state of the utxo view.this
		//entails restoring all of the utxos spent and removing the new
		//onse created by the block.
		err = dbPutUtxoView(dbTx, view)
		if err != nil {
			return err
		}

		//before we delete the spend journal entry for this block ,
		//we will fetch it as is so the indexers can utilize if needed
		stxos, err := dbFetchSpendJournalEntry(dbTx, block)
		if err != nil {
			return err
		}

		//update the transaction spend journal by removing the record that
		//contains all txos spent by the block.
		err = dbRemoveSpendJournalEntry(dbTx, block.Hash())
		if err != nil {
			return err
		}

		//allow the index manager to call each of the currently active
		//optional indexs with the block being disconnected so they can
		//update themselves accordingly.
		if b.indexManager != nil {
			err := b.indexManager.DisconnectBlcok(dbTx, block, stxos)
			if err != nil {
				return err
			}

		}
		return nil
	})

	if err != nil {
		return err
	}

	//prune fully spent entries and mark all entries in the view unmodified
	//now that the modifications have been committed to the database .
	view.commit()

	//this node`s parent is now the end of the best chain.
	b.bestChain.setTip(node.parent)

	// Update the state for the best block.  Notice how this replaces the
	// entire struct instead of updating the existing one.  This effectively
	// allows the old version to act as a snapshot which callers can use
	// freely without needing to hold a lock for the duration.  See the
	// comments on the state variable for more details.
	b.stateLock.Lock()
	b.stateSnapshot = state
	b.stateLock.Unlock()

	// Notify the caller that the block was disconnected from the main
	// chain.  The caller would typically want to react with actions such as
	// updating wallets.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockDisconnected, block)
	b.chainLock.Lock()

	return nil

}

//countSpentoutputs returns the number of utxos the passed block spends.
func countSpentOutputs(block *btcutil.Block) int {
	//exclude the coinbase transaction since it can not spend anything
	var numSpent int
	for _, tx := range block.Transactions()[1:] {
		numSpent += len(tx.MsgTx().TxIn)
	}
	return numSpent
}

//reorganizechain reorganizes the block chain by disconnecting the nodes
//in the detachNodes list and connecting the nodes in the attach list it expects
//that the lists are already in the correct order and are in sync with the
//end of the current best chain.specially .nodes that are being disconnectied
//must be in reverse order(thing of poping them off the end of chain)and
//nodes the are being attached must be in forwards order think pushing them
//onto the end of the chain.)

//this function may modify node statused in the block index without fulusing
//this function must be called with the chain state lock held (for writes).

func (b *BlockChain) reorganizeChain(detachNodes, attachNodes *list.List) error {

	//nothing to do if no reorganize nodes were provided.
	if detachNodes.Len() == 0 && attachNodes.Len() == 0 {
		return nil
	}

	//ensure the provided nodes match the current best chain.
	tip := b.bestChain.Tip()
	if detachNodes.Len() != 0 {
		firstDetachNode := detachNodes.Front().Value.(*blockNode)
		if firstDetachNode.hash != tip.hash {
			return AssertError(fmt.Sprintf("recorganize nodes to detach are "+
				"not for the current best chain -- first detach node %v ,"+
				"current chain %V", &firstDetachNode.hash, &tip.hash))
		}
	}

	//ensure the provided nodes are for the same fork point.
	if attachNodes.Len() != 0 && detachNodes.Len() != 0 {
		firstAttachNode := attachNodes.Front().Value.(*blockNode)
		lastDetachNode := detachNodes.Back().Value.(*blockNode)
		if firstAttachNode.parent.hash != lastDetachNode.parent.hash {
			return AssertError(fmt.Sprintf("reorganize nodes do not have the "+
				"same fork point -- first attach parent %v,last detach"+
				"parent %v", &firstAttachNode.parent.hash, &lastDetachNode.parent.hash))
		}
	}

	//trach the old and new best chains heads.

	oldBest := tip
	newBest := tip

	//all of the blocks to detach and related spend journal entries needed
	//to unspend transaction outputs in the blocks being disconnected must
	//be loaded from the database during the reorg check phase below and
	//then the are needed again when doing the actual database updates .
	//rather than dong two loads .cache the loaded data into these slices.
	detachBlocks := make([]*btcutil.Block, 0, detachNodes.Len())
	detachSpentTxOuts := make([][]SpentTxOut, 0, detachNodes.Len())
	attachBlocks := make([]*btcutil.Block, 0, attachNodes.Len())

	//disconnect all of the blocks back to the point of the fork. this
	//entails loading the blocks and their associated spent txos from the
	//database and using that information to unspend all of the spent txos
	//and remove the utxos created by the blocks.
	view := NewUtxoViewpoint()
	view.SetBestHash(&oldBest.hash)

	for e := detachNodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*blockNode)
		var block *btcutil.Block
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			block, err = dbFetchBlockByNode(dbTx, n)
			return err
		})

		if err != nil {
			return err
		}

		if n.hash != *block.Hash() {
			return AssertError(fmt.Sprintf("detach block node hash %v(hegiht + "+
				"%v) does not match previous parent block hash %v", &n.hash, n.height, block.Hash()))
		}

		// Load all of the utxos referenced by the block that aren't
		// already in the view.
		err = view.fetchInputUtxos(b.db, block)
		if err != nil {
			return err
		}

		// Load all of the spent txos for the block from the spend
		// journal.
		var stxos []SpentTxOut
		err = b.db.View(func(dbTx database.Tx) error {
			stxos, err = dbFetchSpendJournalEntry(dbTx, block)
			return err
		})
		if err != nil {
			return err
		}

		// Store the loaded block and spend journal entry for later.
		detachBlocks = append(detachBlocks, block)
		detachSpentTxOuts = append(detachSpentTxOuts, stxos)

		err = view.disconnectTransactions(b.db, block, stxos)
		if err != nil {
			return err
		}

		newBest = n.parent

	}

	//set the fork point only if there are nodes to attach since otherwire
	//blocks are only being disconnectted and thus there is no fork point.
	var forkNode *blockNode
	if attachNodes.Len() > 0 {
		forkNode = newBest
	}

	//perform several checks to verity each block that needs to be attached
	//to the main chain can be connceted without violating any rules and without
	//actually connecting the block
	//note:these checks could be done directly when connecting a block ,
	//however the downside to that approach is that if any of these checks
	//fail after dicconnecting some blocks or attaching others all of the opreatins
	//have to be rolled back to get the chain back into the state it was befor
	//the rule violation(or other failure).there are as least a couple of ways
	//accomplish that rollback ,but both involve tweaking the chain and /or database .
	//this approach catches these issues before ever modifying the chain.

	for e := attachNodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*blockNode)

		var block *btcutil.Block
		err := b.db.View(func(dbTx database.Tx) error {
			var err error
			block, err = dbFetchBlockByNode(dbTx, n)
			return err
		})
		if err != nil {
			return err
		}

		//store the loaded block for later.
		attachBlocks = append(attachBlocks, block)

		//skip checks if node has alredy been fully validated.although
		//checkconncetblock gets skipped ,we still need to update the utxo
		//view
		if b.index.NodeStatus(n).KnownInvalid() {
			err = view.fetchInputUtxos(b.db, block)
			if err != nil {
				return err
			}

			err = view.connectTransaction(block, nil)
			if err != nil {
				return err
			}

			newBest = n
			continue
		}

		//notice the spent txout details are not requested here and thus
		//will not be generated .this is done because the state is not
		//being immediately wirtten to the database .so it is not needed

		//in the case the block is determinded to be invalid due to a
		//rule violation ,mark it as invalid and mark all of its descendants
		//as having an invalid ancestor.
		err = b.checkConnectBlock(n, block, view, nil)
		if err != nil {
			if _, ok := err.(RuleError); ok {
				b.index.SetStatusFlags(n, statusValidateFailed)
				for de := e.Next(); de != nil; de = de.Next() {
					dn := de.Value.(*blockNode)
					b.index.SetStatusFlags(dn, statusInvalidAncestor)

				}
			}
			return err
		}

		b.index.SetStatusFlags(n, statusValid)
		newBest = n
	}

	//reset the view for the actual connect code below ,this is required becaruse
	//because the view was previously modified when checking if
	// the reorg would be successful and the connection code requires the
	// view to be valid from the viewpoint of each block being connected or
	// disconnected.
	view = NewUtxoViewpoint()
	view.SetBestHash(&b.bestChain.Tip().hash)

	//disconnect blocks from the main chain.
	for i, e := 0, detachNodes.Front(); e != nil; i, e = i+1, e.Next() {
		n := e.Value.(*blockNode)
		block := detachNodes[i]

		//load all of the utxos referenced by the block that are not
		//aleay in the view.

		err := view.fetchInputUtxos(b.db, block)
		if err != nil {
			return err
		}

		//update the view to unspend all of the spent txos and remove
		//utxos created by the block
		err = view.disconnectTransactions(b.db, block, detachSpentTxOuts[i])
		if err != nil {
			return err
		}

		//update the datebase and chain state
		err = b.disconnectBlock(n, block, view)
		if err != nil {
			return err
		}

	}

	//connect the new best chain blocks
	for i, e := 0, attachNodes.Front(); e != nil; i, e = i+1, e.Next() {
		n := e.Value.(*blockNode)
		block := attachBlocks[i]

		//load all of the utxos referenced by the block that are not already
		//in the veiw
		err := view.fetchInputUtxos(b.db, block)
		if err != nil {
			return nil
		}

		//update the view to mark all utxos referenced by the block as spent
		//and add all tranasactions being created by this block to it .also
		//provided an stxos slice so the spent txout details are generated.
		stxos := make([]SpentTxOut, 0, countSpentOutputs(block))
		err = view.connectTransactions(block, &stxos)
		if err != nil {
			return err
		}

		//update the datebase and chain state
		err = b.connectBlock(n, block, view, stxos)
		if err != nil {
			return err
		}

	}
	//log the point where the chain forked and old new best chain heads
	if forkNode != nil {
		log.Infof("reorganize:chain forkd at %v(height %v)", forkNode.hash, forkNode.height)

	}

	log.Infof("recongize:old best chain head was %v(height%v)", &oldBest.hash, oldBest.height)
	log.Infof("REORGANIZE: New best chain head is %v (height %v)",
		newBest.hash, newBest.height)
	return nil

}

//connectbestchain handle connecting the passed block to the chain while
//respecting proper chain selecting according to the chain with the moset
//proof of work .in the typical case . the new block simply extends the main
//chain.however it may also be extending (or creating)a side chain(fork)
//which may or may not end up becoing the main depending on which fork
//cumulatively has the most proof of work .it returns whether or not the block
//end up on the main chain(either due to extending the main chain or caruing a
//a reorganization to become the main ).

//the flags modify the behavior of this fucntion as follows.
//bfffastadd:avoids several expensive transaction validation operations.
//this is useful when using checkpoints.
//this function must be called with the chain state lock held(for wiretes.).
func (b *BlockChain) connectBestChain(node *blockNode, block *btcutil.Block, flags BehaviorFlags) (bool, error) {

	fastAdd := flags&BFFastAdd == BFFastAdd

	flushIndexState := func() {
		//intentionally ignore errors wirting updated node status to DB ,if
		//it fails to writes,it is not end of the world .if the block is valid
		//we flush in connectBlock and if the block is invalid ,the worst that
		//can happen is we revalidate the block after a restart .
		if writeErr := b.index.flushToDB(); writeErr != nil {
			log.Warnf("error flushing block index changes to dis:%v,", writeErr)

		}
	}

	//we are extending the main best chain with a new block .this is the most common
	//case
	parentHash := &block.MsgBlock().Header.PrevBlock
	if parentHash.IsEqual(&b.bestChain.Tip().hash) {
		//skip checks if node has already been fully validated.
		fastAdd = fastAdd || b.index.NodeStatus(node).KnownInvalid()

		//perform several checks to verify the block can be connected
		//to the main without violating any rules and without actually
		//connecting the block.
		view := NewUtxoViewpoint()

		view.SetBestHash(parentHash)
		stxos := make([]SpentTxOut, 0, countSpentOutputs(block))
		if !fastAdd {
			err := b.checkConnectBlock(node, block, view, &stxos)
			if err == nil {
				b.index.SetStatusFlags(node, statusValid)

			} else if _, ok := err.(RuleError); ok {
				b.index.SetStatusFlags(node, statusValidateFailed)
			} else {
				return false, err
			}

			flushIndexState()

			if err != nil {
				return false, err
			}
		}

		//in the fast add case the code to check block connection
		//was skipped. so the utxo view needs to load the referenced
		//utxos .spend them .and add the new utxos being created by
		//this block.
		if fastAdd {
			err := view.fetchInputUtxos(b.db, block)
			if err != nil {
				return false, err
			}

			err = view.connectTransaction(block, &stxos)
			if err != nil {
				return false, err
			}
		}

		//connect the block to the main chain.

		err := b.connectBlock(node, block, view, stxos)
		if err != nil {
			//if we got hit with a rule error .then we will mark that status of the
			//block as invalid and flush the index state to disk before returning with
			//the error
			if _, ok := err.(RuleError); ok {
				b.index.SetStatusFlags(
					node, statusValidateFailed)
			}

			flushIndexState()

			return false, err

		}

		//if this is fast add. or this block node is not yet marked as valid
		//then we will update its status and flush the state to disk again.
		if fastAdd || !b.index.NodeStatus(node).KnownInvalid() {
			b.index.SetStatusFlags(node, statusValid)
			flushIndexState()
		}
		return true, nil

	}

	if fastAdd {
		log.Warnf("fastadd set in the side chain case? %v\n", block.Hash())
	}

	//we are extending (or creating ) a side chain .but the cumulative work
	//for this new side is not enough to make it the new chain.
	if node.workSum.Cmp(b.bestChain.Tip().workSum) <= 0 {
		//log information about how the block is forking the chain
		fork := b.bestChain.findFork(node)
		if fork.hash.IsEqual(parentHash) {
			log.Info("fork ,block %v forks the chain at height %d"+
				"/block %v,but does not cause a reorgainize ",
				node.hash, fork.height, fork.hash)
		} else {
			log.Info("EXTEND FORK:block %v extends a side chain "+
				"which forks the chain at height %d/block %v", node.hash, fork.height,
				fork.hash)
		}
		return false, nil
	}

	//we are extending or creating a side chain and the cumulative work
	//for this new side chain is more than the old best chain ,so this
	//side chain needs to become the main chain,in order to accomplish that
	//find the common ancestor of both sides of the fork .disconnect the
	//blocks that form the new chain to the main chain starting at the common
	//ancenstor(the point where the chain forked )
	detachNodes, attachNodes := b.getReorganizeNodes(node)

	//reorganize the chain
	log.Infof("Reorganize :block %v is causing a reorganize .", node.hash)
	err := b.reorganizeChain(detachNodes, attachNodes)

	//either getreorganizendoes or reorganizechain could have made unsaved
	//changes to the block index ,so flush regardless of whether there was
	//an error .the index would only be dirty if the block failed to connect .
	//so we can innore any errors writing.
	if writeErr := b.index.flushToDB(); writeErr != nil {
		log.Warnf("Error flushing block index changes to disk :%v", writeErr)

	}
	return err == nil, err

}

//iscurrent returns wherther or not the chain believes it is current .several
//factors are used to guess.but the key factors that allow the chain to
//believe it is current are :
//-latest block height is after the latest checkpoint(if enabled)
//-latest block has a timestamp newer than 24 hores ago.

//this function must be called with the chain state lock held(for reads)
func (b *BlockChain) isCurrent() bool {
	//not current if the latest main(best) chain height is before the
	//latest known good checkpoint (when checkpoint are enabled).
	checkpoint := b.LatestCheckkpoint()
	if checkpoint != nil && b.bestChain.Tip().height < checkpoint.Height {
		return false
	}

	//not current is the laste best block has a timestamp before 24 hours
	//ago
	//the chain appears to be current if node of the checks reported otherwise
	minus24Hours := b.timeSource.AdjustedTime().Add(-24 * time.Hour).Unix()
	return b.bestChain.Tip().timestamp >= minus24Hours
}
