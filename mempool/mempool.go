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
	lsatUpdate    int64 //last time pool was updated
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



















