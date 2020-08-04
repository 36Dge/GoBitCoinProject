package mining

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"container/heap"
	"github.com/btcsuite/btcutil"
	"time"
)

const (

	//minhighprioryity is the minimum priority value that allows a
	//transaction to be considred high priority.
	MinHighPriority = btcutil.SatoshiPerBitcoin * 144.0 / 250

	//blockheaderoverhead is the max number of bytes it takes to serivalize
	//a block header and max possible transaction count.
	blockHeaderOverhead = wire.MaxBlockHeaderPayload + wire.MaxVarIntPayload

	//coinbaseflags is added to the coinbase script of a generated block
	//and is used to monitor Bip16 support as well as blocks that are generated vai
	//btcd
	CoinbaseFlags = "/p2SH/btcd"
)

// TxDesc is a descriptor about a transaction in a transaction source along with
// additional metadata.
type TxDesc struct {
	// Tx is the transaction associated with the entry.
	Tx *btcutil.Tx

	// Added is the time when the entry was added to the source pool.
	Added time.Time

	// Height is the block height when the entry was added to the the source
	// pool.
	Height int32

	// Fee is the total fee the transaction associated with the entry pays.
	Fee int64

	// FeePerKB is the fee the transaction pays in Satoshi per 1000 bytes.
	FeePerKB int64
}

//txsoure represent a source of transactions to consider for inclusion in
//new blocks

//the interface contract requires that all of these methods are safe for
//concrurent access with respect to the souce
type TxSource interface {
	//lastupdated returns the last time a trasaction was added to or
	//removed form the soure pool.
	LastUpdated() time.Time

	//miningdesc retuens a slice of mining descriptors for all the
	//transctions in the soure pool.
	MiningDescs() []*TxDesc

	//havetransaction returns whether or not the passed transaction hash
	//exits in the source pool
	HaveTransaction(hash *chainhash.Hash) bool
}

//txpirioitem houses a transaction along with extra information that allows
//the transaction to be prioritized and track depnedencies on the other
//transactions which have not bent mined into a block yet.
type txPrioItem struct {
	tx       *btcutil.Tx
	fee      int64
	priority float64
	feePerKB int64

	//depnedson holds a map of transaction hashes which this one depends
	//on.it will only set when the trasnaction referances other transacion
	// in the source pool and hence must come after them in a block
	dependsOn map[chainhash.Hash]struct{}
}

//txPritorityquencelessfunc describes a function that can be used as a compare
//function for a transacion priority queue.
type txPriorityQueueLessFunc func(*txPriorityQueue, int, int) bool

//txpriorityqueue implements a priority queue of txpriotime elements that
//supports an arbiaray compare function as defined by txpriorityQueeulessfunc
type txPriorityQueue struct {
	lessFunc txPriorityQueueLessFunc
	items    []*txPrioItem
}

//less returns whether the item in the pritority queue with index i should sort
//before the item with index j by defering to the assinged less function.it is part
//of the heap.interface implementation

func (pq *txPriorityQueue) Len() int {
	return len(pq.items)
}

//less returns whether the item in the priority queue with index i should sort
//before the item with index j by deferring to the assined less function .
//it is part of the heap.interface implemation.
func (pq *txPriorityQueue) Less(i, j int) bool {
	return pq.lessFunc(pq, i, j)
}

//swap the item at the passed indices in the priority queuu .it is
//part of the heap.interace implemation.
func (pq *txPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

//push pushed the passed item onto the priority queue. it is part of
//the heap.interface implementation.
func (pq *txPriorityQueue) Push(x interface{}) {
	pq.items = append(pq.items, x.(*txPrioItem))
}

//pop removes the hightst priority item (according to less) form the priorit
//queueu and return it. it is part the heap.interface implemation.
func (pq *txPriorityQueue) Pop() interface{} {
	n := len(pq.items)
	item := pq.items[n-1]
	pq.items[n-1] = nil
	pq.items = pq.items[0 : n-1]
	return item
}

// SetLessFunc sets the compare function for the priority queue to the provided
// function.  It also invokes heap.Init on the priority queue using the new
// function so it can immediately be used with heap.Push/Pop.
func (pq *txPriorityQueue) SetLessFunc(lessFunc txPriorityQueueLessFunc) {
	pq.lessFunc = lessFunc
	heap.Init(pq)
}

//txpqbypriority sorts a txpriorityquueu by transacion pirority and then fees
//per kilobyte.
func txPQByPriority(pq *txPriorityQueue, i, j int) bool {
	//using > here so that pop gives the highest priority item as opposed
	//to the lowest .sort by priority first .then fee.
	if pq.items[i].priority == pq.items[j].priority {
		return pq.items[i].feePerKB > pq.items[j].feePerKB
	}
	return pq.items[i].priority > pq.items[j].priority
}

//txpqbyfee sorst a txpriorityqueue by fees per kilobyte and then transacion
//priority.
func txPQByFee(pq *txPriorityQueue, i, j int) bool {
	//using > here so that pop gives the higest fee item as opposed
	//to the lowest,sort by fee first then priority
	if pq.items[i].feePerKB == pq.items[j].feePerKB {
		return pq.items[i].priority > pq.items[j].priority
	}

	return pq.items[i].feePerKB > pq.items[j].feePerKB

}

//newtxpriorityqueuw returns a new transaction priority queue that reserves the
//passed amount of space for the elements the new priority queue uses either
//the txprqpriority or the txpqbyfee compare function depending on the sortbyfee
//paremater and is already initialized for use with heap.push / pop .the priority
//queue can grow larger than the reserved space.but extra copies of the
//underlying array can be avoided by reserving a sane value.
func newTxPriorityQueue(reserve int, sortByFee bool) *txPriorityQueue {
	pq := &txPriorityQueue{
		items: make([]*txPrioItem, 0, reserve),
	}
	if sortByFee {
		pq.SetLessFunc(txPQByFee)
	} else {
		pq.SetLessFunc(txPQByPriority)
	}
	return pq
}

//blocktemplate hourse a block that has yet to be solved along with additional
//details about the fees and the number of signature operations for each
//transaction in the block.
type BlockTemplate struct {
	//block is a block that is ready to be solve by miners .thus it is
	//completely valid with the exception of satisfying the proof-of-work
	//requirement.
	Block *wire.MsgBlock

	//fees contains the amount of fees each transacion in the generated
	//template pays in base units since the first transacion is the coinbase.
	//the first entry (offset 0) will contain the nagative of the sum of the
	//fees of all other transacion.
	Fees []int64

	//sigopcosts contains the numbers of signature operations each transaction int
	//the generated template performs
	SigOpCost []int64

	//heght is the height at which the block template connects to the main
	//chain.
	Height int32

	//validpayaddress indicates wherher or not the template coinbase pays
	//to an address or its redeemable by anyone see the documentation on
	//newblocktemplate for details on which this can be useful to gennerate
	//template without a coinbase payment address.
	ValidPayAddress bool

	//witnesscommitment is a comminment to the witness data (if any)
	//within the block. this field will only be populted once segregated
	//witness has been activated. and the block contains a transaction which
	//has witness data.

	WitnessCommitment []byte
}

//mergeutxoview adds all of the entries in viewb to viewa.the result is that
//viewa will contain all of its original entries plus all of entries in
//viewb .it will replace any entries in viewb which also exist in viewa
//if the entry in viewa is spent
func mergeUtxoView(viewA *blockchain.UtxoViewpoint, viewB *blockchain.UtxoViewpoint) {
	viewAEntries := viewA.Entries()
	for outpoint, entryB := range viewB.Entries() {
		if entryA, exists := viewAEntries[outpoint]; !exists ||
			entryA == nil || entryA.isSpent() {
			viewAEntries[outpoint] = entryB
		}
	}
}

// standardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks and adds
// the extra nonce as well as additional coinbase flags.
func standardCoinbaseScript(nextBlockHeight int32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(nextBlockHeight)).
		AddInt64(int64(extraNonce)).AddData([]byte(CoinbaseFlags)).
		Script(), nil
}

// createCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func createCoinbaseTx(params *chaincfg.Params, coinbaseScript []byte, nextBlockHeight int32, addr btcutil.Address) (*btcutil.Tx, error) {

	//create the script to pay to the provided payment address if one was
	//specified ,otherwise create a script that allows the coinbase to be
	//redeemable by anyone.
	var pkScript []byte
	if addr != nil {
		var err error
		pkScript, err = txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}

	} else {
		var err error
		scriptBulider := txscript.NewScriptBulider()
		pkScript, err = scriptBulider.AddOp(txscript.OP_TRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		//coinbase tranactions have no inputs ,so previous output is zrio
		//hash and max index.

		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		SignatureScript:  coinbaseScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    blockchain.CalcBlockSubsidy(nextBlockHeight, params),
		PkScript: pkScript,
	})

	return btcutil.NewTx(tx), nil

}

//spendtransaction updates the passed view by marking the inputs to the
//passed transaction as spent.it also adds all outputs in the passed transaction
//which are not provably unspendable as avaialbe unspent transaction oputputs.
func spendtransaction(utxoView *blockchain.UtxoViewpoint, tx *btcutil.Tx, height int32) error {
	for _, txIn := range tx.MsgTx().TxIn {
		entry := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if entry != nil {
			entry.Spend()
		}
	}

	utxoView.AddTxOuts(tx, height)
	return nil
}

//logskippeddeps logs any depnedencyes which are also skipped as a result
//of skipping a transaction while generating a block template at the
//trace level.
func logSkippedDeps(tx *btcutil.Tx, deps map[chainhash.Hash]*txPrioItem) {
	if deps == nil {
		return
	}

	for _, item := range deps {
		log.Tracef("skipping tx %s since it depneds on %s\n", item.tx.Hash(), tx.Hash())
	}
}

//minimummediantime returns the minimum allowed timestamp for a block buliding on the
//end of the provided best chain.in particular .it is one second after the median
//timestamp of the laste several blocks per the chain conseus rules
func MinimumMedianTime(chainState *blockchain.BestState) time.Time {
	return chainState.MedianTime.Add(time.Second)
}

//medianadjustedtime returns the current time adjuested to ensure it
//is at least one second after the median timestamp of the last several
//blocks per the chain consensus ruels.
func medianAdjustTime(chainState *btcutil.BestState, timeSource blockchain.MedianTimeSource) time.Time {

	//the timestamp for the block must not be before the median timestamp
	//of the last several blocks thus ,choose the maximum between the current
	//time and one second after
	newTimestamp := timeSource.AdjustedTime()
	minTimestamp := MinimumMedianTime(chainState)
	if newTimestamp.Before(minTimestamp) {
		newTimestamp = minTimestamp
	}

	return newTimestamp

}
//blktmplgenerator providers a type that can be used to generate block template
//based on a given minning policy and source of trnasaction to choose from.
//it also houses addtional state required in order to ensure the templates
//are bulit on top of the current best chain and andhere to the consensus rules.
type BlkTmplGenerator struct {
	Policy *Policy
	chainParams *chaincfg.Params
	TxSource TxSource
	chain *blockchain.BlockChain
	timeSource blockchain.MedianTimeSource
	sigCache    *txscript.SigCache
	hashCache   *txscript.HashCache
}

// NewBlkTmplGenerator returns a new block template generator for the given
// policy using transactions from the provided transaction source.
//
// The additional state-related fields are required in order to ensure the
// templates are built on top of the current best chain and adhere to the
// consensus rules.
func NewBlkTmplGenerator(policy *Policy, params *chaincfg.Params,
	txSource TxSource, chain *blockchain.BlockChain,
	timeSource blockchain.MedianTimeSource,
	sigCache *txscript.SigCache,
	hashCache *txscript.HashCache) *BlkTmplGenerator {

	return &BlkTmplGenerator{
		policy:      policy,
		chainParams: params,
		txSource:    txSource,
		chain:       chain,
		timeSource:  timeSource,
		sigCache:    sigCache,
		hashCache:   hashCache,
	}
}

//

















































