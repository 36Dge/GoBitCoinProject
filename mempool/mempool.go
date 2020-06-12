package mempool

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/btcjson"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/mining"
	"BtcoinProject/wire"
	"container/list"
	"fmt"
	"github.com/btcsuite/btcutil"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (

	//defaultblockprioritysize is the default size in bytes for high-priority/low-fee
	//transactions.it is used to help determine which are allowed into the mempool and
	//consequently affects their relay and inclusion when generating block templeates

	DefaultBlockPrioritySize = 50000

	//orphanttl is the maximum amount of time an orphan is allowed to stay
	//in the orphan pool before it expires and evicted during the next scan
	orphanTTL = time.Minute * 15

	//orphanexpirescaninterval is the minimum amount of time in between scans
	//of the orphan pool to evict expired transactions .
	orphanExpireScanInterval = time.Minute * 5

	//maxRbfsequence is the maximum sequence number an input can use to
	//singal that the transaction spending it can be replaced using the
	//replace-by -fee policy
	MaxRBFSequence = 0xfffffffd

	//maxReplaceevictions is the maximum number of transactions that can be
	//evicted from the mempool when accepting a transaction replacement.
	MaxReplacementEvictions = 100
)

// Tag represents an identifier to use for tagging orphan transactions.  The
// caller may choose any scheme it desires, however it is common to use peer IDs
// so that orphans can be identified by which peer first relayed them.

type Tag uint64

//config is a descriptor containing the memory pool configuration
type Config struct {

	//policy defines the various mempool configuration options related to policy
	Policy Policy

	//chainparams indentifies which chain parameters the txpool is associated with.
	ChainParams *chaincfg.Params

	//fetchutxoview defines the function to use to fetch unspent transaction
	//output information.
	FetchUtxoView func(*btcutil.Tx) (*blockchain.UtxoViewpoint, error)

	//bestheight defines the function to use to acess the block height of
	//the current best chain.
	BestHeight func() int32

	//mediantimepast defines the function to use in order to access the
	//median time past calculated form point-of-view of the current chain
	//tip within the best chain
	MedianTimePast func() time.Time

	//calcsequencelock defines the function to use in order to generate
	//the current sequence lock for the given tranasaction using the passed
	//utxo view
	CalcSequenceLock func(*btcutil.Tx, *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error)

	//isdeploymentactive returns true if the target deploymentIDs is active
	//and false otherwise.the mempool uses this function to gauge
	//if transaction using new to be soft-forded rules should be allowed
	//into the mempool or not .
	IsDeploymentActive func(deploymentID uint32) (bool, error)

	//sigcache define a signature cache to use
	SigCache *txscript.SigCache

	//hashCache defines the transaction hash mid-state cache to use.
	HashCache *txscript.HashCache

	// AddrIndex defines the optional address index instance to use for
	// indexing the unconfirmed transactions in the memory pool.
	// This can be nil if the address index is not enabled.
	AddrIndex *indexers.AddrIndex

	// FeeEstimatator provides a feeEstimator. If it is not nil, the mempool
	// records all new transactions it observes into the feeEstimator.
	FeeEstimator *FeeEstimator
}

//policy houses the policy (configuration paramaters)which is used to
//control the mempool
type Policy struct {
	//maxtxversion is the transaction version that the mempool should
	//accept ,all transaction above this version are rejeceted as mon-standard
	MaxTxVersion int32

	//disablerelaypriority defines whether to relay free or low-fee
	//transaction that do not have enough priority to be relayed
	DisableRelayPriority bool

	//acceptnonstd difines whether to accept non-standard transactions if
	//true.non-standard transactions will be accepted into the mempool.
	//otherwise.all non-standard transactions will be rejected.
	AcceptNonStd bool

	//freetxrelaylimit defines the given amount in thousands of bytes
	//per minute that transactions with no fee are rate limited to.
	FreeTxRelayLimit float64

	//maxorphantxs is the maxium number of orphan transactions that
	//can be queued.
	MaxOrphanTxs int

	//maxorphantxsize is the maximum size allowed for orphan transactions
	//this helps prenent memory exhaustion attacks from sending a lot of
	//of big orphans .
	MaxOrphanTxSize int

	//maxsigopcostpertx is the cumulative maximum cost of all the signature
	//operations in a single tranaaction we will relay or mine .it is a fraction
	//of the max signature operations for a block.
	MaxSigOpCostPerTx int

	//minrelaytxfee define the minimum transaction fee in btc/kb to be
	//considered a non-zero fee.
	MinRelayTxFee btcutil.Amount

	//rejectreplacement.if true .rejects accepting replacement
	//transactions using the replace-by-fee signing policy into
	//the mempool
	RejectReplacement bool
}

//txdesc is a descriptor containing a transaction in the mempool along with
//additional metadata.
type TxDesc struct {
	mining.TxDesc

	//startingpriority is the prrority of the tranaction when it was added to the pool
	StartingPriority float64
}

//orphan is normal transaction that refercences an anscetor transacion
//that is not yet avaibable.it also contains additional information related
//to it such as an expireaction time to help prevent caching the orphan forver

type orphanTx struct {
	tx         *btcutil.Tx
	tag        Tag
	expiration time.Time
}

//txpool is used as a sourece of transactions that need to be mined into blocks
//and relayed to others peers .it is safe for concurent access form multipile peers
type TxPool struct {
	//the following variable must only be used atomically.
	lastUpdated   int64 //last time pool was updated
	mtx           sync.RWMutex
	cfg           Config
	pool          map[chainhash.Hash]*TxDesc
	orphans       map[chainhash.Hash]*orphanTx
	orphansByPrev map[wire.OutPoint]map[chainhash.Hash]*btcutil.Tx
	outpoints     map[wire.OutPoint]*btcutil.Tx
	pennyTotal    float64 //exponentially decaying total for penny spends
	lastPennyUnix int64   //unix time of last `penny spend`
	//nextexpiscan is the time after which the orphan pool will be scanned in order
	//to evict orphans.this is not a hard dealine as the scan will only run when
	//an orphan is added to the pool as opposed to on an unconditionsl timer
	nextExpireScan time.Time
}

//ensure the txpool type implements the mining.txsource interface
var _ mining.TxSource = (*TxPool)(nil)

//removeorphan is the internal function which implements the public
//removeorphan see the comment for removeorphan for more details.

func (mp *TxPool) removeOrphan(tx *btcutil.Tx, removeRedeemers bool) {

	//nothing to do if passed tx is not an orphan.

	txHash := tx.Hash()
	otx, exists := mp.orphans[*txHash]
	if !exists {
		return
	}

	//remove the reference from the privious orphan index.
	for _, txIn := range otx.tx.MsgTx().TxIn {
		orphans, exists := mp.orphansByPrev[txIn.PreviousOutPoint]
		if exists {
			delete(orphans, *txHash)
			//remove the map entry altogether if there are no 
			//longer any orphan which depend on it.
			if len(orphans) == 0 {
				delete(mp.orphansByPrev, txIn.PreviousOutPoint)
			}
		}

	}

	//remove any orphans that redeem outputs from this one if required
	if removeRedeemers {
		prevOut := wire.OutPoint{Hash: *txHash}
		for txOutIdx := range tx.MsgTx().TxOut {
			prevOut.Index = uint32(txOutIdx)
			for _, orphan := range mp.orphansByPrev[prevOut] {
				mp.removeOrphan(orphan, true)
			}
		}

	}

	//remove the transaction from the orphan pool
	delete(mp.orphans, *txHash)

}

//removeorphan removes the passed orphan transaction from the orphan pool and
//previous orphan index.
//this function is safe for concurrent access.
func (mp *TxPool) RemoveOrphan(tx *btcutil.Tx) {
	mp.mtx.Lock()
	mp.removeOrphan(tx, false)
	mp.mtx.Unlock()
}

//removeorphanbytag removes all orphan transactions tagged with the provided
//identifier
//this function is safe for concurent access
func (mp *TxPool) RemoveOrphansByTag(tag Tag) uint64 {
	var numEvicted uint64
	mp.mtx.Lock()
	for _, otx := range mp.orphans {
		if otx.tag == tag {
			mp.removeOrphan(otx.tx, true)
			numEvicted++
		}
	}
	mp.mtx.Unlock()
	return numEvicted

}

//limitsnumorphans limits the number of orphan transaction by evcting a
//random orphan if adding a new one would cause it to overflow the max
//allowed .
//this function must be called with mempool lock held(for writer).
func (mp *TxPool) limitNumOrphans() error {
	//scan through the orphan pool and remove any expired orphans when it is
	//time .this is done for efficiency so the san only happens
	//periodically instead of on everty orphan added to the pool
	if now := time.Now(); now.After(mp.nextExpireScan) {
		origNumOrphans := len(mp.orphans)
		for _, otx := range mp.orphans {
			if now.After(otx.expiration) {
				//remove redeemers too because the missing
				//paraments are very unlikey to ever materilaize
				//since the orphan has already been around more
				//than long enough for them to be delievered .
				mp.removeOrphan(otx.tx, true)
			}
		}

		//set next expireation scan to occur after the scan interval.
		mp.nextExpireScan = now.Add(orphanExpireScanInterval)

		numOrphans := len(mp.orphans)
		if numeExpired := origNumOrphans - numOrphans; numeExpired > 0 {
			log.Debug("Expire %d %s (remaining:%d)", numeExpired,
				pickNoun(numeExpired, "orphan", "orphans"), numOrphans)
		}

	}

	//nothing to do if adding anoter orphan will not cause the pool to exceed the limit.
	if len(mp.orphans)+1 <= mp.cfg.Policy.MaxOrphanTxs {
		return nil
	}
	//remove a random entry from map.for most compilers .go's range statement interates
	//staring at a random item although that is not 100%.guaranteed by the spec.  The iteration order
	// is not important here because an adversary would have to be
	// able to pull off preimage attacks on the hashing function in
	// order to target eviction of specific entries anyways.
	for _, otx := range mp.orphans {
		//do not remove redeemers in the case of a random eviction since 
		//it is quite possible it might be needed again shortly,
		mp.removeOrphan(otx.tx, false)
		break
	}
	return nil

}

//addorphan adds an orphan transaction to hte orphan pool.
//this function must be called with the mempool lock held(for writes)
func (mp *TxPool) addOrphan(tx *btcutil.Tx, tag Tag) {
	//nothing to do if no orphans are allowed 
	if mp.cfg.Policy.MaxOrphanTxs <= 0 {
		return
	}

	//limit the number orphan transactions to prevent memory exhaustion.
	//this will periodically remove any expired orphans and evict a random 
	//orphans if space is still needed.
	mp.limitNumOrphans()

	mp.orphans[*tx.Hash()] = &orphanTx{
		tx:         tx,
		tag:        tag,
		expiration: time.Now().Add(orphanTTL),
	}

	for _, txIn := range tx.MsgTx().TxIn {
		if _, exists := mp.orphansByPrev[txIn.PreviousOutPoint]; !exists {
			mp.orphansByPrev[txIn.PreviousOutPoint] = make(map[chainhash.Hash]*btcutil.Tx)
		}
		mp.orphansByPrev[txIn.PreviousOutPoint][*tx.Hash()] = tx
	}

	log.Debugf("Stored orphan transaction %v(total :%d)", tx.Hash(), len(mp.orphans))

}

//maybeaddorphan potentially adds an orphan to the orphan pool.
//this function must be called with the mempool lock held (for writes).
func (mp *TxPool) maybeAddOrphan(tx *btcutil.Tx, tag Tag) error {
	// Ignore orphan transactions that are too large.  This helps avoid
	// a memory exhaustion attack based on sending a lot of really large
	// orphans.  In the case there is a valid transaction larger than this,
	// it will ultimtely be rebroadcast after the parent transactions
	// have been mined or otherwise received.
	//
	// Note that the number of orphan transactions in the orphan pool is
	// also limited, so this equates to a maximum memory used of
	// mp.cfg.Policy.MaxOrphanTxSize * mp.cfg.Policy.MaxOrphanTxs (which is ~5MB
	// using the default values at the time this comment was written).

	serializedLen := tx.MsgTx().SerializeSize()
	if serializedLen > mp.cfg.Policy.MaxOrphanTxSize {
		str := fmt.Sprintf("orphan transaction size of %d bytes is "+
			serializedLen, mp.cfg.Policy.MaxOrphanTxSize)
		return txRuleError(wire.RejectNonstandard, str)
	}

	//add the orphan if the none of the above disqualified it.
	mp.addOrphan(tx, tag)

	return nil

}

//removeorphansdoubleSpends remove all orphans which spend outputs spnets
//by the passed transaction from the orphan pool .removing those orphasn then
//leads to removing all orphans which rley on them.recursively.this is
//necessary when a transaction is added to main pool because it spend
//outputs that orphans also spend.
//this function must be called with the mempool lock held(for writes)
func (mp *TxPool) removeOrphanDoubleSpends(tx *btcutil.Tx) {
	msgTx := tx.MsgTx()
	for _, txIn := range msgTx.TxIn {
		for _, orphan := range mp.orphansByPrev[txIn.PreviousOutPoint] {
			mp.removeOrphan(orphan, true)
		}
	}
}

//is transactioninpool returns whether or not passed transaction already
//exist in the mian pool.

//this function must be called with the mempool lock held (for reads)
func (mp *TxPool) isTransactionInPool(hash *chainhash.Hash) bool {
	if _, exists := mp.pool[*hash]; exists {
		return true
	}
	return false
}

//istransactioninpool returns whether or not the passed transaction already
// exists in the main pool.
//
// This function is safe for concurrent access.
func (mp *TxPool) IsTransactionInPool(hash *chainhash.Hash) bool {

	//protect concurrent access
	mp.mtx.RLock()
	inPool := mp.isTransactionInPool(hash)
	mp.mtx.RUnlock()
	return inPool

}

//isorphaninpool returns whether or not the passed transaction already e
//in the orphan pool
//this function must be called with the mempool lock held (for reads)
func (mp *TxPool) isOrphanInPool(hash *chainhash.Hash) bool {
	if _, exists := mp.orphans[*hash]; exists {
		return true
	}
	return false
}

// IsOrphanInPool returns whether or not the passed transaction already exists
// in the orphan pool.
//
// This function is safe for concurrent access.

func (mp *TxPool) IsOrphanInPool(hash *chainhash.Hash) bool {
	//protect concurrent access
	mp.mtx.RLock()
	inPool := mp.isOrphanInPool(hash)
	mp.mtx.RUnlock()
	return inPool
}

//havetransaction returns whether or not the passed transaction already
//exists in the main pool or in the orphan pool.
//this function must be called with the mempool lock held(for reads)
func (mp *TxPool) haveTransaction(hash *chainhash.Hash) bool {
	//protect concurent access
	mp.mtx.RLock()
	haveTx := mp.haveTransaction(hash)
	mp.mtx.RUnlock()

	return haveTx
}

//remove transaction is the internal fucnction which implements the public
//removetransaction. see the comment for removetransaction for more details

//this functions must be called with the mempool lock held (for writes)
func (mp *TxPool) removeTransaction(tx *btcutil.Tx, removeRedeemers bool) {
	txHash := tx.Hash()
	if removeRedeemers {
		//remove any transaction which relay on this one
		for i := uint32(0); i < uint32(len(tx.MsgTx().TxOut)); i++ {
			prevOut := wire.OutPoint{Hash: *txHash, Index: i}
			if txRedeemer, exists := mp.outpoints[prevOut]; exists {
				mp.removeTransaction(txRedeemer, true)
			}
		}

	}

	//remove the transaction if needed

	if txDesc, exists := mp.pool[*txHash]; exists {
		//remove unconfirmed address index entries accsciated with the transaction
		//if enabled.
		if mp.cfg.AddrIndex != nil {
			mp.cfg.AddrIndex.RemoveUnconfirmedTx(txHash)
		}

		//mark the referenced outpoints as unspent by the pool.
		for _, txIn := range txDesc.Tx.MsgTx().TxIn {
			delete(mp.outpoints, txIn.PreviousOutPoint)
		}
		delete(mp.pool, *txHash)
		atomic.StoreInt64(&mp.lastUpdated, time.Now().Unix())

	}

}

// RemoveTransaction removes the passed transaction from the mempool. When the
// removeRedeemers flag is set, any transactions that redeem outputs from the
// removed transaction will also be removed recursively from the mempool, as
// they would otherwise become orphans.
//
// This function is safe for concurrent access.
func (mp *TxPool) RemoveTransaction(tx *btcutil.Tx, removeRedeemers bool) {
	// Protect concurrent access.
	mp.mtx.Lock()
	mp.removeTransaction(tx, removeRedeemers)
	mp.mtx.Unlock()
}

//removedoubleseends removes all transactions which outputs spent by
//the passed transactoin from the mempool .removging those transaction
//then leads to removing all transaction which relay on them .recursively.
//this is necessary when a block is connected to the main chain beacuse
//the blcok may contain transaction which were priviously unknown to
//the memory pool
//this function is safe for concurrent access
func (mp *TxPool) RemoveBoubleSpends(tx *btcutil.Tx) {
	//protect concurent access
	mp.mtx.Lock()
	for _, txIn := range tx.MsgTx().TxIn {

		if txRedeemer, ok := mp.outpoints[txIn.PreviousOutPoint]; ok {
			if !txRedeemer.Hash().IsEqual(tx.Hash()) {
				mp.removeTransaction(txRedeemer, true)
			}
		}
	}
	mp.mtx.Unlock()
}

//addtransaction adds the passed transaction to the mempory pool.it
//should not be called directly as it doesn't perform any validation .
//this is a helper for maybeaccpttransaction
//this is must be called with the mempool lock held (for writes)
func (mp *TxPool) addTransaction(utxoView *blockchain.UtxoViewpoint, tx *btcutil.Tx, height int32, fee int64) *TxDesc {
	//add the transaction to the pool and mark the referenced outputs
	//as spent by the pool

	txD := &TxDesc{
		TxDesc: mining.TxDesc{
			Tx:       tx,
			Added:    time.Now(),
			Height:   height,
			Fee:      fee,
			FeePerKB: fee * 1000 / GetTxVirtualSize(tx),
		},
		StartingPriority: mining.CalcPriority(tx.MsgTx(), utxoView, height),
	}

	mp.pool[*tx.Hash()] = txD
	for _, txIn := range tx.MsgTx().TxIn {
		mp.outpoints[txIn.PreviousOutPoint] = tx
	}
	atomic.StoreInt64(&mp.lastUpdated, time.Now().Unix())

	//add unconfirmed address index entries associated with the transaction
	//if enable
	if mp.cfg.AddrIndex != nil {
		mp.cfg.AddrIndex.AddUnconfirmedTx(tx, utxoView)
	}

	//remove this tx for fee estimation if enabled.
	if mp.cfg.FeeEstimator != nil {
		mp.cfg.FeeEstimator.ObserveTransaction(txD)
	}

	return txD

}

// checkPoolDoubleSpend checks whether or not the passed transaction is
// attempting to spend coins already spent by other transactions in the pool.
// If it does, we'll check whether each of those transactions are signaling for
// replacement. If just one of them isn't, an error is returned. Otherwise, a
// boolean is returned signaling that the transaction is a replacement. Note it
// does not check for double spends against transactions already in the main
// chain.
//this function must be called with the mempool lock held (for reads)
func (mp *TxPool) checkPoolDoubleSpend(tx *btcutil.Tx) (bool, error) {
	var isReplacement bool
	for _, txIn := range tx.MsgTx().TxIn {
		conflict, ok := mp.outpoints[txIn.PreviousOutPoint]
		if !ok {
			continue
		}

		//reject the transaction if we do not accept replacement
		//transaction or if it does not signal replacement
		// Reject the transaction if we don't accept replacement
		// transactions or if it doesn't signal replacement.
		if mp.cfg.Policy.RejectReplacement ||
			!mp.signalsReplacement(conflict, nil) {
			str := fmt.Sprintf("output %v already spent by "+
				"transaction %v in the memory pool",
				txIn.PreviousOutPoint, conflict.Hash())
			return false, txRuleError(wire.RejectDuplicate, str)
		}
		isReplacement = true
	}

	return isReplacement, nil
}

// signalsReplacement determines if a transaction is signaling that it can be
// replaced using the Replace-By-Fee (RBF) policy. This policy specifies two
// ways a transaction can signal that it is replaceable:
//
// Explicit signaling: A transaction is considered to have opted in to allowing
// replacement of itself if any of its inputs have a sequence number less than
// 0xfffffffe.
//
// Inherited signaling: Transactions that don't explicitly signal replaceability
// are replaceable under this policy for as long as any one of their ancestors
// signals replaceability and remains unconfirmed.
//
// The cache is optional and serves as an optimization to avoid visiting
// transactions we've already determined don't signal replacement.
//
// This function MUST be called with the mempool lock held (for reads).

func (mp *TxPool) signalsReplacement(tx *btcutil.Tx, cache map[chainhash.Hash]struct{}) bool {
	//if a cache was not provided ,we will intialize one now to ust
	//for the recursive calls.
	if cache == nil {
		cache = make(map[chainhash.Hash]struct{})
	}

	for _, txIn := range tx.MsgTx().TxIn {
		if txIn.Sequence <= MaxRBFSequence {
			return true
		}

		hash := txIn.PreviousOutPoint.Hash
		unconfirmedAncestor, ok := mp.pool[hash]
		if !ok {
			continue
		}

		// If we've already determined the transaction doesn't signal
		// replacement, we can avoid visiting it again.
		if _, ok := cache[hash]; ok {
			continue
		}

		if mp.signalsReplacement(unconfirmedAncestor.Tx, cache) {
			return true
		}

		// Since the transaction doesn't signal replacement, we'll cache
		// its result to ensure we don't attempt to determine so again.
		cache[hash] = struct{}{}

	}

	return false

}

//txancestors returns all of the unconfimed anastors of given
//transaction.given transaction a,b, and where c spneds b and
// B spends A,
// A and B are considered ancestors of C.

//the cache is optional and serves as an optimization to avoid visting
//transaction we have already determined ancestors of .

//this function must be called with the mempool lock held (for reads)
func (mp *TxPool) txAncestors(tx *btcutil.Tx, cache map[chainhash.Hash]map[chainhash.Hash]*btcutil.Tx) map[chainhash.Hash]*btcutil.Tx {
	//if a cache was not provided,we will initialize one now to use for
	//the recursive calls
	if cache == nil {
		cache = make(map[chainhash.Hash]map[chainhash.Hash]*btcutil.Tx)
	}

	ancestors := make(map[chainhash.Hash]*btcutil.Tx)
	for _, txIn := range tx.MsgTx().TxIn {

		parent, ok := mp.pool[txIn.PreviousOutPoint.Hash]
		if !ok {
			continue
		}
		ancestors[*parent.Tx.Hash()] = parent.Tx

		//determine if the ancestors of this ancestor have already been
		//computed.if they haven.t ,we will do so now and cache them to use
		//them later on if necessay
		moreAncestors, ok := cache[*parent.Tx.Hash()]
		if !ok {
			moreAncestors = mp.txAncestors(parent.Tx, cache)
			cache[*parent.Tx.Hash()] = moreAncestors
		}

		for hash, ancestor := range moreAncestors {
			ancestors[hash] = ancestor

		}
	}

	return ancestors
}

// txDescendants returns all of the unconfirmed descendants of the given
// transaction. Given transactions A, B, and C where C spends B and B spends A,
// B and C are considered descendants of A. A cache can be provided in order to
// easily retrieve the descendants of transactions we've already determined the
// descendants of.
//
// This function MUST be called with the mempool lock held (for reads).
func (mp *TxPool) txDescendants(tx *btcutil.Tx, cache map[chainhash.Hash]map[chainhash.Hash]*btcutil.Tx) map[chainhash.Hash]*btcutil.Tx {
	// if a cache was not provided,we will initialize one to use for the
	//recursive calls .
	if cache == nil {
		cache = make(map[chainhash.Hash]map[chainhash.Hash]*btcutil.Tx)
	}

	//we will go throught all of the outputs of the transaction to determine
	//if they are spent by any other mempool transactions.

	descendants := make(map[chainhash.Hash]*btcutil.Tx)
	op := wire.OutPoint{Hash: *tx.Hash()}
	for i := range tx.MsgTx().TxOut {
		op.Index = uint32(i)
		descendant, ok := mp.outpoints[op]
		if !ok {
			continue
		}
		descendants[*descendant.Hash()] = descendant

		//determine if the descendants of this descendant have already
		//been computed. if they have not ,we will do so now and cache
		//them to use them later on if necessary .
		moreDescendants, ok := cache[*descendant.Hash()]
		if !ok {
			moreDescendants = mp.txDescendants(descendant, cache)
			cache[*descendant.Hash()] = moreDescendants

		}

		for _, moreDescendant := range moreDescendants {
			descendants[*moreDescendant.Hash()] = moreDescendant
		}

	}
	return descendants
}

//txconficts returns all of the unconfirmed transaction that would become
//conflicts if we were to accept the given transaction into the mempool .an
//unconfirmed confiict is known as a transaction that spends an output already
//spent by a different transacion within the mempool.any descendants of the
//these transactions are also considered conflicts as they would no longer exists
//these are generally not allowed except for transactions that singal RBF support.
func (mp *TxPool) txConflicts(tx *btcutil.Tx) map[chainhash.Hash]*btcutil.Tx {
	conflicts := make(map[chainhash.Hash]*btcutil.Tx)
	for _, txIn := range tx.MsgTx().TxIn {
		conflict, ok := mp.outpoints[txIn.PreviousOutPoint]
		if !ok {
			continue
		}

		conflicts[*conflict.Hash()] = conflict
		for hash, descendant := range mp.txDescendants(conflict, nil) {
			conflicts[hash] = descendant
		}

	}
	return conflicts
}

//checkspend checks whether the passed outpoint is already spent by a transaction in
//the mempool.if that is the case the spending transaction will be retuened .
//if not nil will be retruned.
func (mp *TxPool) CheckSpend(op wire.OutPoint) *btcutil.Tx {
	mp.mtx.RLock()
	txR := mp.outpoints[op]
	mp.mtx.RUnlock()

	return txR
}

//fetchinputUtxos loads details about the inut transactions referenced
//by the passed transaction first it loads the details form the viewpoint
//of the main chain.then it adjusts them based upon the contents of the
//
//this function MUST be called the mempool lock held (for reads.)
func (mp *TxPool) fetchInputUtxos(tx *btcutil.Tx) (*blockchain.UtxoViewpoint, error) {
	utxoView, err := mp.cfg.FetchUtxoView(tx)
	if err != nil {
		return nil, err
	}

	//attempt to populate any missing inputs from the transaction pool
	for _, txIn := range tx.MsgTx().TxIn {
		prevOut := &txIn.PreviousOutPoint
		entry := utxoView.LookupEntry(*prevOut)
		if entry != nil && !entry.isSpent() {
			continue
		}

		if poolTxDesc, exists := mp.pool[prevOut.Hash]; exists {
			//addtxout ignores out of range index values ,so it is
			//safe to call without bounds checking here.
			utxoView.AddTxOut(poolTxDesc.Tx, prevOut.Index, mining.UnminedHeight)
		}

	}
	return utxoView, nil
}

//fetchtransaction returns the requested transaction from the tranasaction
//pool .this is only fetchs from the main transaction pool and does not
//include orphans.
//this function is safe for concurrent access
func (mp *TxPool) FetchTransaction(txHash *chainhash.Hash) (*btcutil.Tx, error) {
	//protect concurrent access
	mp.mtx.RLock()
	txDesc, exists := mp.pool[*txHash]
	mp.mtx.RUnlock()
	if exists {
		return txDesc.Tx, nil
	}
	return nil, fmt.Errorf("transaction is not in the pool")
}

//validatereplacement determines wherther a transaction is deemed as a valid
//replacement of all its conflicts according to the rbf policy .if it is valid
//no error is returned.otherwise an error is returned indicating what went wrong.

//this function must be called with the mempool lock held (for read)
func (mp *TxPool) validateReplacement(tx *btcutil.Tx, txFee int64) (map[chainhash.Hash]*btcutil.Tx, error) {

	//first ,we will make sure the set of conflicting transaction doesn,t
	//exceed the maximum allowed
	conflicts := mp.txConflicts(tx)
	if len(conflicts) > MaxReplacementEvictions {
		str := fmt.Sprintf("replacement transaction %v evicts more "+"transacations than permitted :max is %v ,evitss %v ",
			tx.Hash(), MaxReplacementEvictions, len(conflicts))
		return nil, txRuleError(wire.RejectNonstandard, str)
	}

	//the set of confilicts (transactions we will replace )and anscestors should not
	//overlap ,otherwise the replacement would be spending an output that no longer exists.
	for ancestorHash := range mp.txDescendants(tx, nil) {
		if _, ok := conflicts[ancestorHash]; !ok {
			continue
		}
		str := fmt.Sprintf("replacement transaction %v spends parent "+
			"transaction %v ", tx.Hash(), ancestorHash)
		return nil, txRuleError(wire.RejectInvalid, str)
	}

	//the replacemnt should have a higher fee rate than each of the
	//conflicting transactions and a higher absolute fee than the fee
	//sum of all the conflicting transactions.

	//we usually don,t want to accept replacement with lower fee rates
	//than what they replaced as that would lower the fee rate of the next
	//block.requring that the fee rate always be increeaed is also an easy-to-reason
	//about way to prevent dos attacks via replacements.
	var (
		txSize           = GetTxVirtualSize(tx)
		txFeeRate        = txFee * 1000 / txSize
		conflictsFee     int64
		conflictsParents = make(map[chainhash.Hash]struct{})
	)
	for hash, conflicts = range conflicts {
		if txFeeRate <= mp.pool[hash].FeePerKB {
			str := fmt.Sprintf("replacement transaction %v has an "+
				"insufficient fee rate : needs more than %v "+
				"has %v ", tx.Hash(), mp.pool[hash].FeePerKB, txFeeRate)
			return nil, txRuleError(wire.RejectInsufficientFee, str)
		}

		conflictsFee += mp.pool[hash].Fee

		//we wll track each conflict,s parents to ensure the replacement
		//is not spending any new unconfirmed inputs
		for _, txIn := range conflicts.MsgTx().TxIn {
			conflictsParents[txIn.PreviousOutPoint.Hash] = struct{}{}
		}

	}

	//it should also have an absoulte fee greater than all of the transaction
	//it intends to repalce and pay for its own bnadwidth.
	//which is determined by our minimum relay fee.
	minFee := calcMinRequiredTxRelayFee(txSize, mp.cfg.Policy.MinRelayTxFee)
	if txFee < conflictsFee+minFee {
		str := fmt.Sprintf("replacement transaction %v has an "+
			"insufficient absolute fee :nees %v ,has %v ",
			tx.Hash(), conflictsFee+minFee, txFee)
		return nil, txRuleError(wire.RejectInsufficientFee, str)
	}

	// finanlly ,it should not spend any new unconfirmed outputs ,other than
	//the ones already inculuded in the parents of the conflicting transaction
	//it will replace
	for _, txIn := range tx.MsgTx().TxIn {
		if _, ok := conflictsParents[txIn.PreviousOutPoint.Hash]; ok {
			continue
		}

		//confirmed outpus are valid to spend in the replacement .

		if _, ok := mp.pool[txIn.PreviousOutPoint.Hash]; !ok {
			continue
		}
		str := fmt.Sprintf("replacement transaction spends new"+
			"uncomfirmed input %v not found in conflicting "+
			"transactions,", txin.PreviousOutPoint)
		return nil, txRuleError(wire.RejectInvalid, str)

	}
	return conflicts, nil

}

// maybeaccepttransaction is the internal function which implements the public
//maybeaccepttransaction. see the comment for mayaccepttranascion for more details
//this function MUST be called with the mempool lock held (for writes).

func (mp *TxPool) maybeAcceptTransaction(tx *btcutil.Tx, isNew, rateLimit, rejectDupOrphans bool) ([]*chainhash.Hash, *TxDesc, error) {

	txHash := tx.Hash()

	//if a transaction has witness data ,and segwit isn,t active yet.if segwit isn,t active yet.
	//then we won,t accept it into the mempool as
	//it can,t be mined yet.
	if tx.MsgTx().HasWitness() {
		segwitActive, err := mp.cfg.IsDeploymentActive(chaincfg.DeploymentSegwit)
		if err != nil {
			return nil, nil, err
		}

		if !segwitActive {

			str := fmt.Sprintf("transaction %v has witness data ,"+
				"but segwit is not active yet ", txHash)
			return nil, nil, txRuleError(wire.RejectNonstandard, str)
		}
	}

	//don't accept the transaction if it already exists in the pool .this
	//applies to orphan transactions as well when the project duplicate
	//orphans flag is set .this check is intended to be q quick to weed out
	//duplicates
	if mp.isTransactionInPool(txHash) || (rejectDupOrphans && mp.isOrphanInPool(txHash)) {
		str := fmt.Sprintf("already have transaction %v ", txHash)
		return nil, nil, txRuleError(wire.RejectDuplicate, str)
	}

	//perform preliminary sanity checks on the transaction. this makes
	//use of blockchain which contains the invariant rules for what transaction
	//are allowed into blocks .
	err := blockchain.CheckTransactionSanity(tx)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}

	//a standlone transaction must not be coinbase transaction
	if blockchain.IsCoinBase(tx) {
		str := fmt.Sprintf("transaction %v is an indivinal coinbase ",
			txHash)
		return nil, nil, txRuleError(wire.RejectInvalid, str)
	}

	//get the current height of the main chain.a standalone transaction
	//will be mined into the next block at best ,so its height is at least
	//one more than the current height.
	bestHeight := mp.cfg.BestHeight()
	nextBlockHeight := bestHeight + 1

	medianTimePast := mp.cfg.MedianTimePast()

	//don't allow non-standard transaction if the network parameters
	//forbid their acceptance.
	if !mp.cfg.Policy.AcceptNonStd {
		err = checkTransactionStandard(tx, nextBlockHeight, medianTimePast, mp.cfg.Policy.MinRelayTxFee, mp.cfg.Policy.MaxTxVersion)
	}

	if err != nil {
		//attempt to extract a reject code from the error so it can be retained.
		//when not possible ,fall back to a non strandard error.
		rejectCode, found := extractRejectCode(err)
		if !found {
			rejectCode = wire.RejectNonstandard
		}
		str := fmt.Sprintf("transaction %v is not standard :%v", txHash, err)
		return nil, nil, txRuleError(rejectCode, str)
	}

	//the transaction may not use any of the same outputs as other
	//transactions already in the pool as that would ultimately result
	//in a doube spend .unless those transactions signal for RBf,this
	//check is intended to be quick and therefore only detects double spends within
	// the transaction pool itself. The transaction could still be double
	// spending coins from the main chain at this point. There is a more
	// in-depth check that happens later after fetching the referenced
	// transaction inputs from the main chain which examines the actual
	// spend data and prevents double spends.

	isReplacement, err := mp.checkPoolDoubleSpend(tx)
	if err != nil {
		return nil, nil, err
	}

	//fetch all of the unspent transaction outpouts referenced by the
	//inputs to this transaction this function also attempts to fetch
	//the transacton iteself to be used for detecting a duplcate transaction
	//without needing to to a seprate lookup.

	utxoView, err := mp.fetchInputUtxos(tx)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}

	// do not allow the transaction if it exits in the main chain and is not
	//not already fully spent.

	prevOut := wire.OutPoint{Hash: *txHash}
	for txOutIdx := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		entry := utxoView.LookupEntry(prevOut)
		if entry != nil && entry.IsSpent() {
			return nil, nil, txRuleError(wire.RejectDuplicate, "transaction already exists")
		}
		utxoView.RemoveEntry(prevOut)
	}

	//transaction is an orphan any of the referenced transaction outputs
	//don,t exist or are already spent. adding orphans to the orphans pool
	//is not handled by this function.and the caller should use maybeAddorphan
	//if this behavior is desired.
	var missingParents []*chainhash.Hash
	for outpoint, entry := range utxoView.Entries() {
		if entry == nil || entry.IsSpent() {
			//must make a copy of the hash here since the iterator
			//is replaced and taking its addredd directory would
			//result in all of the entried pointing to the same
			//memory loaction and thus all be the final hash.
			hashCopy := outpoint.Hash
			missingParents = append(missingParents, &hashCopy)

		}
	}
	if len(missingParents) > 0 {
		return missingParents, nil, nil
	}

	//dont allow the transaction into the mempool unless its sequence
	//lock is active ,meaning that it will be allowed into the next block
	//withe respect to its defined relative lock time.
	sequenceLock, err := mp.cfg.CalcSequenceLock(tx, utxoView)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}
	if !blockchain.SequenceLockActive(sequenceLock, nextBlockHeight, medianTimePast) {
		return nil, nil, txRuleError(wire.RejectNonstandard, "tansaction is sequence locks on inputs not met")
	}

	//perform several checks on the transaction inputs using the invarinant
	//rules in blockchain for what transactions are allowed into blcoks.
	//also returns the fees associated with the transaction which willa be
	//used later.
	txFee, err := blockchain.CheckTransactionInputs(tx, nextBlockHeight, utxoView, mp.cfg.ChainParams)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}

	//don,t allowe transaction with non-standard inputs if the network
	//parameters forbid their acceptance.
	if !mp.cfg.Policy.AcceptNonStd {
		err := checkInputsStandard(tx, utxoView)
		if err != nil {
			//attempt to extract a reject code from the error so
			//it can be retained. when not possible,fall back to
			//a non strandard error.
			rejectCode, found := extractRejectCode(err)
			if !found {
				rejectCode = wire.RejectNonstandard
			}
			str := fmt.Sprintf("transaction %v has a non-standard "+
				"input: %v ", txHash, err)
			return nil, nil, txRuleError(rejectCode, str)
		}
	}

	//note:if you modify this code to accept non-strandard transactions,
	//you should add code here to check that the transaction does a reasonable
	//number of ECDSA signature verifications.

	//don,t allow transaction with an excessive number of signature operations
	//which would result in making it possible to mine .since the coinbase adderess
	//iteself can contoin signature operations, the
	// maximum allowed signature operations per transaction is less than
	// the maximum allowed signature operations per block.
	sigOpCost, err := blockchain.GetSigOpCost(tx, false, utxoView, true, true)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}
	if sigOpCost > mp.cfg.Policy.MaxSigOpCostPerTx {
		str := fmt.Sprintf("transaction %v sigop cost is too high :%d > %d ", txHash, sigOpCost, mp.cfg.Policy.MaxSigOpCostPerTx)

		return nil, nil, txRuleError(wire.RejectNonstandard, str)
	}

	//don,t allow transactions with fees too low to get into a mined block.

	//most miners allow a free transaction area in blocks they mine to go
	//alongside the area used for high-priority transactions as well as transaction
	//with fees. a transaction size of up to 1000 bytes is calculated below on its
	//own would encourage several small transactions to avoid fees rather than one
	//single larger transaction
	// which is more desirable.  Therefore, as long as the size of the
	// transaction does not exceeed 1000 less than the reserved space for
	// high-priority transactions, don't require a fee for it.

	serializedSize := GetTxVirtualSize(tx)
	minFee := calcMinRequiredTxRelayFee(serializedSize, mp.cfg.Policy.MinRelayTxFee)

	if serializedSize >= (DefaultBlockPrioritySize-1000) && txFee < minFee {
		str := fmt.Sprintf("transacion %v has %d fees which is under "+
			"the required amout of %d ", txHash, txFee, minFee)
		return nil, nil, txRuleError(wire.RejectInsufficientFee, str)
	}

	//require that free transactions have sufficient priority to be mined
	//in the next block. transacions which are being added back to the mempool
	//from blocks that have been disconnected druing a reorg are exempted.
	if isNew && !mp.cfg.Policy.DisableRelayPriority && txFee < minFee {
		currentPriority := mining.CalcPriority(tx.MsgTx(), utxoView, nextBlockHeight)
		if currentPriority <= mining.MinHighPriority {
			str := fmt.Sprintf("transaction %v has insufficient "+"priority (%g <= %g)", txHash, currentPriority, mining.MinHighPriority)
			return nil, nil, txRuleError(wire.RejectInsufficientFee, str)
		}
	}

	//free to relay transaions are rate limited to prevent penny-flooding with
	//tiny transacions as a form of attack.
	if rateLimit && txFee < minFee {
		nowUnix := time.Now().Unix()

		//dacay passed data with an exponentially decaying - 10 minute
		//window - matched bitcoind handling.

		mp.pennyTotal *= math.Pow(1.0-1.0/600.0, float64(nowUnix-mp.lastPennyUnix))
		mp.lastPennyUnix = nowUnix

		//are we still over the limit?
		if mp.pennyTotal >= mp.cfg.Policy.FreeTxRelayLimit*10*1000 {
			str := fmt.Sprintf("transacion %v has benn rejected "+
				"by the rate limiter due to low fees", txHash)
			return nil, nil, txRuleError(wire.RejectInsufficientFee, str)
		}
		oldTotal := mp.pennyTotal

		mp.pennyTotal += float64(serializedSize)
		log.Tracef("rate limit:curTotal %v ,nextTotal:%v,"+
			"limit %v ", oldTotal, mp.pennyTotal, mp.cfg.Policy.FreeTxRelayLimit*10*1000)

	}

	//if the transaction has any conflicts and we have made it this far ,then
	//we are processing a potential replacement
	var conflicts map[chainhash.Hash]*btcutil.Tx
	if isReplacement {
		conflicts, err = mp.validateReplacement(tx, txFee)
		if err != nil {
			return nil, nil, err
		}
	}

	//vertify cypto singature for each input and reject the transaction if
	//any don,t verify.
	err = blockchain.ValidateTransactionScripts(tx, utxoView,
		txscript.StandardVerifyFlags, mp.cfg.SigCache,
		mp.cfg.HashCache)
	if err != nil {
		if cerr, ok := err.(blockchain.RuleError); ok {
			return nil, nil, chainRuleError(cerr)
		}
		return nil, nil, err
	}

	//now that we have deemed the transaction as valid ,we can add it to the
	//mempool, if it ended up replacing any transacions .we will remove them
	//first .
	for _, conflict := range conflicts {
		log.Debugf("Replacing transaction %v (fee_rate= %v sat/kb)"+
			"with %v(fee_rate= %v sat/kb)\n", conflict.Hash(), mp.pool[*conflict.Hash()].FeePerKB, tx.Hash(), txFee*1000/serializedSize)

		//the confilict set should already include the descendants for
		//each one so we do not need to remove the redeemers whigin
		//this call as they will be removed enentually.
		mp.removeTransaction(conflict, false)
	}

	txD := mp.addTransaction(utxoView, tx, bestHeight, txFee)
	log.Debugf("accepted transaction %v (pool size:%v)", txHash, len(mp.pool))

	return nil, txD, nil
}

// MaybeAcceptTransaction is the main workhorse for handling insertion of new
// free-standing transactions into a memory pool.  It includes functionality
// such as rejecting duplicate transactions, ensuring transactions follow all
// rules, detecting orphan transactions, and insertion into the memory pool.
//
// If the transaction is an orphan (missing parent transactions), the
// transaction is NOT added to the orphan pool, but each unknown referenced
// parent is returned.  Use ProcessTransaction instead if new orphans should
// be added to the orphan pool.
func (mp *TxPool) MaybeAcceptTransaction(tx *btcutil.Tx, isNew, rateLimit bool) ([]*chainhash.Hash, *TxDesc, error) {
	// Protect concurrent access.
	mp.mtx.Lock()
	hashes, txD, err := mp.maybeAcceptTransaction(tx, isNew, rateLimit, true)
	mp.mtx.Unlock()

	return hashes, txD, err
}

//processorphans is the internal function which implemnts the public
//precessorphans .see the comment for processorphans for more details

func (mp *TxPool) processOrphans(acceptedTx *btcutil.Tx) []*TxDesc {
	var acceptedTxns []*TxDesc

	//start with processing at least the passed transaction
	processList := list.New()
	processList.PushBack(acceptedTx)
	for processList.Len() > 0 {
		//pop the transaction to precess from the front of the list
		firstElement := processList.Remove(processList.Front())
		processItem := firstElement.(*btcutil.Tx)

		prevOut := wire.OutPoint{Hash: *processItem.Hash()}
		for txOutIdx := range processItem.MsgTx().TxOut {

			// Look up all orphans that redeem the output that is
			// now available.  This will typically only be one, but
			// it could be multiple if the orphan pool contains
			// double spends.  While it may seem odd that the orphan
			// pool would allow this since there can only possibly
			// ultimately be a single redeemer, it's important to
			// track it this way to prevent malicious actors from
			// being able to purposely constructing orphans that
			// would otherwise make outputs unspendable.
			//
			// Skip to the next available output if there are none.

			prevOut.Index = uint32(txOutIdx)
			orphans, exists := mp.orphansByPrev[prevOut]
			if !exists {
				continue
			}

			//potentially accept an orphan into the tx pool
			for _, tx := range orphans {
				missing, txD, err := mp.maybeAcceptTransaction(
					tx, true, true, false)

				if err != nil {
					//the orphan is now invalid ,so there in no way any other
					//orphans which redeem any of its outputs can be accepted
					//remove them
					mp.removeOrphan(tx, true)
					break
				}

				//transaction is still an orphan .try the next orphan which
				//redeems this output
				if len(missing) > 0 {
					continue
				}

				//transaction was accepted into the mian pool
				//add it to the list of accepted transaction
				//that are no longer orphans ,remove it from
				//the orphan pool,and add it to the list of
				//transactions to process so any orphans that
				//depend on it are handled too.
				acceptedTxns = append(acceptedTxns, txD)
				mp.removeOrphan(tx, false)
				processList.PushBack(tx)

				//only one transction for this outpoint can
				//be accepted,so the rest are now double spends
				//and are removed later.

				break
			}
		}

	}
	//recursively remove any orphans that also redeem any outpus redeemed
	//by the accepted transactions since those are now definitive double
	//spends.

	mp.removeOrphanDoubleSpends(acceptedTx)
	for _, txD := range acceptedTxns {
		mp.removeOrphanDoubleSpends(txD.Tx)
	}
	return acceptedTxns

}

//processorphans determines if there are any orphans which depned on the passed
//transaction hash(it is possible that they are no longer orphans)and protentially
//accepts them to the memory pool.it repaeats the process for the newly accepted transaction
//(to detect further orphans which may no longer be orphash )until there are no more.

// It returns a slice of transactions added to the mempool.  A nil slice means
// no transactions were moved from the orphan pool to the mempool.
//
// This function is safe for concurrent access.
func (mp *TxPool) ProcessOrphans(acceptedTx *btcutil.Tx) []*TxDesc {
	mp.mtx.Lock()
	acceptedTxns := mp.processOrphans(acceptedTx)
	mp.mtx.Unlock()

	return acceptedTxns
}

//processtranscion is the main workhorse for handling insertion of new
//free-standing transactions into the memory pool.it includes functionally
//such as rejecting dupliacte transactions,ensuring transactions follow all
//rules,orphan trnasaction handling,and insertion into the memeory pool.

//it returns a slice of transactions added to the mempool.when the erros
//is nil,the list will inculude the passed transaction iteself along
//with any additional orphans transactions that were added as a result of
//the passed one being accepted.

//this function is safe for concurrent access.
func (mp *TxPool) ProcessTransaction(tx *btcutil.Tx, allowOrphan, rateLimit bool, tag Tag) ([]*TxDesc, error) {
	log.Tracef("Processing transaction %v", tx.Hash())

	//protect concurrent access
	mp.mtx.Lock()
	defer mp.mtx.Unlock()

	//potentially accept the transaction to the mempool .
	missingParents, txD, err := mp.maybeAcceptTransaction(tx, true, rateLimit, true)
	if err != nil {
		return nil, err
	}

	if len(missingParents) == 0 {
		//accept any orphan transactions that depned on this
		//transaction (they may no longer be orphans if all inputs
		//are now availbale)and repeat for those accepted transactions
		//until there are eno more.
		newTxs := mp.processOrphans(tx)
		acceptedTxs := make([]*TxDesc, len(newTxs)+1)

		//add the parent transation first so remote nodes
		//do not add orphans
		acceptedTxs[0] = txD
		copy(acceptedTxs[1:], newTxs)

		return acceptedTxs, nil

	}

	//the transaction is an orphan (has inputs missing).reject
	//it if the flag to allow orphans is not set.
	if !allowOrphan {
		//only use the first missing parent transaction in the error message
		//
		//note :rejectduplicate is really not an accurate
		//reject code here.but it matches the reference
		//implementation and there is not a better choice due
		//to the limited number of reject codes.missing
		//inputs is assumed to mean they are already spent
		//which is not really always the case.
		str := fmt.Sprintf("orphan transaction %v references "+
			"outputs of unkonwn or fully-spent"+
			"transaction %v", tx.Hash(), missingParents[0])
		return nil, txRuleError(wire.RejectDuplicate, str)

	}

	//protentially add the orphan transaction to the orphan pool.
	err = mp.maybeAddOrphan(tx, tag)
	return nil, err

}

//count returns the number of transaction in the main pool.it does not
//inculude the orphan pool

//this fucntion is safe for concurrent access
func (mp *TxPool) Count() int {
	mp.mtx.RLock()
	count := len(mp.pool)
	mp.mtx.RUnlock()

	return count
}

//txhashed returns a slice of hashes for all of the trnasaction in the mempool
func (mp *TxPool) TxHashes() []*chainhash.Hash {
	mp.mtx.RLock()
	hashes := make([]*chainhash.Hash, len(mp.pool))
	i := 0
	for hash := range mp.pool {
		hashCopy := hash
		hashes[i] = &hashCopy
		i++
	}
	mp.mtx.RUnlock()
	return hashes
}

//txdeses returns a slice of descriptors for all the transactions in the pol
//the descriptors are to be treated as read only.

func (mp *TxPool) TxDescs() []*TxDesc {
	mp.mtx.RLock()
	descs := make([]*TxDesc, len(mp.pool))

	i := 0
	for _, desc := range mp.pool {
		descs[i] = desc
		i++
	}
	mp.mtx.RUnlock()
	return descs
}

//miningDeScs returns a slice of mining descriptors for all the transactions
//in the pool

//this is part of the mining .txsource interface implementation and is safe
//for concurrent access as required by the interface contract.
func (mp *TxPool) MiningDescs() []*mining.TxDesc {
	mp.mtx.RLock()
	descs := make([]*mining.TxDesc, len(mp.pool))
	i := 0
	for _, desc := range mp.pool {
		descs[i] = &desc.TxDesc
		i++
	}
	mp.mtx.RUnlock()
	return descs
}

//rawremoveverbose returns all of the entries in the mempool as a fully
//populated btcjson result

//this function is safe concurrent access.
func (mp *TxPool) RawMempoolVerbose() map[string]*btcjson.GetRawMempoolVerboseResult {

	mp.mtx.RLock()
	defer mp.mtx.RUnlock()

	result := make(map[string]*btcjson.GetRawMempoolVerboseResult, len(mp.pool))
	bestHeight := mp.cfg.BestHeight()

	for _, desc := range mp.pool {

		//calculate the current priority based on the inputs to
		//the transaction .use zero if one of more of the input
		//transactions can,t be found for some reason.
		tx := desc.Tx
		var currentPriority float64
		utxos, err := mp.fetchInputUtxos(tx)
		if err == nil {
			currentPriority = mining.CalcPriority(tx.MsgTx(), utxos, bestHeight+1)
		}

		mpd := &btcjson.GetRawMempoolVerboseResult{
			Size:             int32(tx.MsgTx().SerializeSize()),
			Vsize:            int32(GetTxVirtualSize(tx)),
			Weight:           int32(blockchain.GetTransactionWeight(tx)),
			Fee:              btcutil.Amount(desc.Fee).ToBTC(),
			Time:             desc.Added.Unix(),
			Height:           int64(desc.Height),
			StartingPriority: desc.StartingPriority,
			CurrentPriority:  currentPriority,
			Depends:          make([]string, 0),
		}

		for _, txIn := range tx.MsgTx().TxIn {
			hash := &txIn.PreviousOutPoint.Hash
			if mp.haveTransaction(hash) {
				mpd.Depends = append(mpd.Depends,
					hash.String())
			}
		}

		result[tx.Hash().String()] = mpd
	}
	return result
}

//lastupdated returns the last time a transaction was added to or remove from
//the main pool.it does not include the orphan pool.

func(mp *TxPool)LastUpdated() time.Time{
	return time.Unix(atomic.LoadInt64(&mp.lastUpdated),0)

}

//new returns a new memory pool for validating and storing standalone
//transactions until they are mined into a block.
func New(cfg *Config)*TxPool{
	return &TxPool{
		cfg:            *cfg,
		pool:           make(map[chainhash.Hash]*TxDesc),
		orphans:        make(map[chainhash.Hash]*orphanTx),
		orphansByPrev:  make(map[wire.OutPoint]map[chainhash.Hash]*btcutil.Tx),
		nextExpireScan: time.Now().Add(orphanExpireScanInterval),
		outpoints:      make(map[wire.OutPoint]*btcutil.Tx),
	}
}

//over



















