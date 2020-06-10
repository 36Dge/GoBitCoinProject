package mempool

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/mining"
	"BtcoinProject/wire"
	"fmt"
	"github.com/btcsuite/btcutil"
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
	conflicts := map.txConflicts(tx)
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
