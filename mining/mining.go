package mining

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"bytes"
	"container/heap"
	"fmt"
	"github.com/btcsuite/btcutil"
	"html/template"
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
	policy      *Policy
	chainParams *chaincfg.Params
	TxSource    TxSource
	chain       *blockchain.BlockChain
	timeSource  blockchain.MedianTimeSource
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

// NewBlockTemplate returns a new block template that is ready to be solved
// using the transactions from the passed transaction source pool and a coinbase
// that either pays to the passed address if it is not nil, or a coinbase that
// is redeemable by anyone if the passed address is nil.  The nil address
// functionality is useful since there are cases such as the getblocktemplate
// RPC where external mining software is responsible for creating their own
// coinbase which will replace the one generated for the block template.  Thus
// the need to have configured address can be avoided.
//
// The transactions selected and included are prioritized according to several
// factors.  First, each transaction has a priority calculated based on its
// value, age of inputs, and size.  Transactions which consist of larger
// amounts, older inputs, and small sizes have the highest priority.  Second, a
// fee per kilobyte is calculated for each transaction.  Transactions with a
// higher fee per kilobyte are preferred.  Finally, the block generation related
// policy settings are all taken into account.
//
// Transactions which only spend outputs from other transactions already in the
// block chain are immediately added to a priority queue which either
// prioritizes based on the priority (then fee per kilobyte) or the fee per
// kilobyte (then priority) depending on whether or not the BlockPrioritySize
// policy setting allots space for high-priority transactions.  Transactions
// which spend outputs from other transactions in the source pool are added to a
// dependency map so they can be added to the priority queue once the
// transactions they depend on have been included.
//
// Once the high-priority area (if configured) has been filled with
// transactions, or the priority falls below what is considered high-priority,
// the priority queue is updated to prioritize by fees per kilobyte (then
// priority).
//
// When the fees per kilobyte drop below the TxMinFreeFee policy setting, the
// transaction will be skipped unless the BlockMinSize policy setting is
// nonzero, in which case the block will be filled with the low-fee/free
// transactions until the block size reaches that minimum size.
//
// Any transactions which would cause the block to exceed the BlockMaxSize
// policy setting, exceed the maximum allowed signature operations per block, or
// otherwise cause the block to be invalid are skipped.
//
// Given the above, a block generated by this function is of the following form:
//
//   -----------------------------------  --  --
//  |      Coinbase Transaction         |   |   |
//  |-----------------------------------|   |   |
//  |                                   |   |   | ----- policy.BlockPrioritySize
//  |   High-priority Transactions      |   |   |
//  |                                   |   |   |
//  |-----------------------------------|   | --
//  |                                   |   |
//  |                                   |   |
//  |                                   |   |--- policy.BlockMaxSize
//  |  Transactions prioritized by fee  |   |
//  |  until <= policy.TxMinFreeFee     |   |
//  |                                   |   |
//  |                                   |   |
//  |                                   |   |
//  |-----------------------------------|   |
//  |  Low-fee/Non high-priority (free) |   |
//  |  transactions (while block size   |   |
//  |  <= policy.BlockMinSize)          |   |
//   -----------------------------------  --
func (g *BlkTmplGenerator) NewBlockTemplate(payToAddress btcutil.Address) (*BlockTemplate, error) {
	//extend the most recently known best block.
	best := g.chain.BestSnapshot()
	nextBlockHeight := best.Height + 1

	//create a standard coinbase transaction paying to the provided
	//address.note:the coinbase value will be updated to include the
	//fees from the selected transacions later after they have actully
	//been selected .it is created here to detect any errors early before
	//protentially doing a lot of work below .the extra nonce helps ensure
	//the transaction is not duplicate transaction(paying the same value to )
	//same public key address would otherise be an indential transaction for
	//block version 1)

	extraNonce := uint64(0)
	coinbaseScript, err := standardCoinbaseScript(nextBlockHeight, extraNonce)
	if err != nil {
		return nil, err
	}
	coinbaseTx, err := createCoinbaseTx(g.chainParams, coinbaseScript, nextBlockHeight, payToAddress)
	if err != nil {
		return nil, err
	}
	coinbaseSigOpCost := int64(blockchain.CountSigOps(coinbaseTx)) * blockchain.WitnessScaleFactor

	//get the current source transactions and create a priority queue to
	//hold the trnasaction which are ready for inclusion into a block along
	//with some priority related and fee metadata. reserve the same number of
	//items that are availalbe for the priority quueue.also choose the initail
	//sort order for the priority queue based on whether or not there is an
	//area allocated for high-priority transaction.
	sourceTxns := g.TxSource.MiningDescs()
	sortedByFee := g.policy.BlockPrioritySize == 0
	priorityQueue := newTxPriorityQueue(len(sourceTxns), sortedByFee)

	//create a slice to hold the transaaction to be included in the
	//generated block with reserved space. also create a utxo view to
	//house all of the input transactions so mutiple lookups can be avoid
	blockTxns := make([]*btcutil.Tx, 0, len(sourceTxns))
	blockTxns = append(blockTxns, coinbaseTx)
	blockUtxos := blockchain.NewUtxoViewpoint()

	//dependers is used to track transacions which depend on anoter
	//transaction in the source pool. this ,in conjunction with the
	//depnedon map kept with each depneds transaction helps quickly
	//determine which dependent transacions are now eligible for inblusion
	//in the block once each transaction has been included.
	dependers := make(map[chainhash.Hash]map[chainhash.Hash]*txPrioItem)

	//create slices to hold the fees and number of signatrue operations
	//for each of the selected transactions and add an entry for the coinbase
	//this allows the code below to simply append details about a transaction
	//as it it selected for inclusion  in the final block. however,since the
	//total fees arenot konr yet .use a dummy value for the coinbase fee which
	// will be updated later.
	txFees := make([]int64, 0, len(sourceTxns))
	txSigOpCosts := make([]int64, 0, len(sourceTxns))
	txFees = append(txFees, -1) //updated once known
	txSigOpCosts = append(txSigOpCosts, coinbaseSigOpCost)

	log.Debugf("considering %d transaction for inclusin to new block ", len(sourceTxns))

mempoolLoop:
	for _, TxDesc := range sourceTxns {
		//a block can not have more than one coinbase or contain
		//non-finallized transactions.
		tx := TxDesc.Tx
		if blockchain.IsCoinBase(tx) {
			log.Tracef("skiping coinbase tx %s", tx.Hash())
			continue
		}

		if !blockchain.IsFinalizedTransaction(tx, nextBlockHeight, g.timeSource.AdjustedTime()) {
			log.Tracef("skipping non-finalized tx%s", tx.Hash())
			continue
		}

		//fetch all of the utxos referenced by the this tranasction.
		//note:this intentionally does not fetch inputs from the mempool
		//since a transaction which depends on other transaction in the
		//mempool must come after those depnedencies in the final generated
		//block.
		utxos, err := g.chain.FetchUtxoView(tx)
		if err != nil {
			log.Warnf("unable to fetch utxo view for tx %s:%v", tx.Hash(), err)
			continue
		}

		//setup dependencies for any transactions which reference other
		//transactions in the mempool so they can be properly ordered below.
		prioItem := &txPrioItem{tx: tx}
		for _, txIn := range tx.MsgTx().TxIn {
			originHash := &txIn.PreviousOutPoint.Hash
			entry := utxos.LookupEntry(txIn.PreviousOutPoint)
			if entry == nil || entry.IsSpent() {
				if !g.TxSource.HaveTransaction(originHash) {
					log.Tracef("skipping tx %s because it"+
						"reference unspent output %s"+
						"which in not available", tx.Hash(), txIn.PreviousOutPoint)
					continue mempoolLoop
				}

				//the transaction is referencing another transaction in the source pool
				//so setup an ordering depnedency.
				deps, exists := dependers[*originHash]
				if !exists {
					deps = make(map[chainhash.Hash]*txPrioItem)
					dependers[*originHash] = deps
				}
				deps[*prioItem.tx.Hash()] = prioItem
				if prioItem.dependsOn == nil {
					prioItem.dependsOn = make(map[chainhash.Hash]struct{})
				}

				prioItem.dependsOn[*originHash] = struct{}{}

				//skip the check below .we already know the referenced transaction is
				//available.
				continue

			}
		}

		//calcuate the final transation priority using the input
		//value age sum well as the adjust transaction size.the
		//formula is :sum(inputvalue * inputage) /adjustdTxsize
		prioItem.priority = CalcPriority(tx.MsgTx(), utxos, nextBlockHeight)

		//calculate the fee in satoshi/kb
		prioItem.feePerKB = TxDesc.FeePerKB
		prioItem.fee = TxDesc.Fee

		//add the transaction to the priority queue to mark it ready
		//for inclusion in the block unless it has depnedencies.
		if prioItem.dependsOn == nil {
			heap.Push(priorityQueue, prioItem)
		}

		//merge the referenced outputs form the input transactions to
		//this transaction into the block utxo view .this allows the
		//code below to avoid a second lookup.
		mergeUtxoView(blockUtxos, utxos)
	}

	log.Tracef("priority queue len %d,depender len %d", priorityQueue.Len(), len(dependers))

	//the strating block size size is the size of the block header plus the
	//max possible transaction count size.plus the size of the coinbase transaction
	blockWeight := uint32((blockHeaderOverhead * blockchain.WitnessScaleFactor) +
		blockchain.GetTransactionWeight(coinbaseTx))
	blockSigOpCost := coinbaseSigOpCost
	totalFees := int64(0)

	//query the version bits states to see if segwit has been activeted,if
	//so then this means that we will include any transactions with witness
	//data in the mempool.and also add the witness commitment as an op_return
	//output in the coinbase transaction.
	segwitState, err := g.chain.ThresholdState(chaincfg.DeploymentSegwit)
	if err != nil {
		return nil, err
	}
	segwitActive := segwitState == blockchain.ThresholdActive

	witnessIncluded := false

	//chose which transactions make into the block.
	for priorityQueue.Len() > 0 {

		//grab the highest priority (or highest fee per kilobyte depeding on the
		//sort order ) transaction.
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		tx := prioItem.tx

		switch {
		//if segregated witness has not been activated yet. then we should not
		//include any witness transaction in the block.
		case !segwitActive && tx.HasWitness():
			continue

		//otherwise ,keep track of if we have included a trnasaction
		//with witness data or not. if so. then we will need to include
		//the witness commitment as the last output in the coinbase trnasaction.

		case segwitActive && !witnessIncluded && tx.HasWitness():
			//if we are about to include a transacion bearing witness data
			//then we will also need to include a witness commitment in the
			//coinbase transaction. therefore we acoout for the addiatiional
			//weight within the block with model coinbase tx with a witness
			//commitment.
			coinbaseCopy := btcutil.NewTx(coinbaseTx.MsgTx().Copy())
			coinbaseCopy.MsgTx().TxIn[0].Witnesss = [][]byte{
				bytes.Repeat([]byte("a"),
					blockchain.CoinbaseWitnessDataLen),
			}
			coinbaseCopy.MsgTx().AddTxOut(&wire.TxOut{
				PkScript: bytes.Repeat([]byte("a"),
					blockchain.CoinbaseWitnessPkScriptLength),
			})

			//in order to accurately account for the weight addition due
			//to this coinbase trasnaction,we will add the difference of
			//the transaction before and after the addition of the commitment to
			//the block weight.

			weightDiff := blockchain.GetTransactionWeight(coinbaseCopy) -
				blockchain.GetTransactionWeight(coinbaseTx)

			blockWeight += uint32(weightDiff)

			witnessIncluded = true

		}

		//grab any transactions which depend on this one.
		deps := dependers[*tx.Hash()]

		//enforce maximum block size. also check for overflow.
		txWeight := uint32(blockchain.GetTransactionWeight(tx))
		blockPlusTxWeight := blockWeight + txWeight

		if blockPlusTxWeight < blockWeight || blockPlusTxWeight >= g.policy.BlockMaxWeight {
			log.Tracef("skipping tx %s because it would exceed "+
				"the max block weight", tx.Hash())
			logSkippedDeps(tx, deps)
			continue
		}

		//enfore maximum signature operation cost per block ,also check for overlaow

		sigOpCost, err := blockchain.GetSigOpCost(tx, false, blockUtxos, true, segwitActive)
		if err != nil {
			log.Tracef("skipping tx %s due to error in "+
				"getsigopcost:%v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}

		if blockSigOpCost+int64(sigOpCost) < blockSigOpCost ||
			blockSigOpCost+int64(sigOpCost) > blockchain.MaxBlockSigOpsCost {
			log.Tracef("skipping tx %s because it would "+"exceed the maximum sigoops per block", tx.Hash())
			logSkippedDeps(tx, deps)
			continue
		}

		//skip free transactions once the block is larger than the minimum block size
		if sortedByFee && prioItem.feePerKB < int64(g.policy.TxMinFreeFee) &&
			blockPlusTxWeight >= g.policy.BlockMinWeight {
			log.Tracef("Skipping tx %s with feePerKB %d "+
				"< TxMinFreeFee %d and block weight %d >= "+
				"minBlockWeight %d", tx.Hash(), prioItem.feePerKB,
				g.policy.TxMinFreeFee, blockPlusTxWeight,
				g.policy.BlockMinWeight)
			logSkippedDeps(tx, deps)
			continue
		}

		//prioritize by fee per kilobyte once block is larger than the priority
		//size or there are no more high-priority transaction.
		if !sortedByFee && (blockPlusTxWeight >= g.policy.BlockPrioritySize ||
			prioItem.priority <= MinHighPriority) {
			log.Tracef("Switching to sort by fees per "+
				"kilobyte blockSize %d >= BlockPrioritySize "+
				"%d || priority %.2f <= minHighPriority %.2f",
				blockPlusTxWeight, g.policy.BlockPrioritySize,
				prioItem.priority, MinHighPriority)

			sortedByFee = true
			priorityQueue.SetLessFunc(txPQByFee)

			//put the transaction back into the priority queue and
			//skip it so it is re-prioritzed by fee if it wont fit
			//fit into the high-priority section or the priority is
			//too low .otherwise this transaction will be the final
			//one in the high-priority section .so just fall through
			//to the code below so it is added now.
			if blockPlusTxWeight > g.policy.BlockPrioritySize ||
				prioItem.priority < MinHighPriority {
				heap.Push(priorityQueue, prioItem)
				continue
			}
		}

		//ensure the transaction inputs pass all of the necesary preconditions before
		//allowing it to be added to the block
		_, err := blockchain.CheckTransactionInputs(tx, nextBlockHeight, blockUtxos, g.chainParams)
		if err != nil {
			log.Tracef("skipping tx %s due to error in "+"checktransactioninputs:%v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}
		err = blockchiain.ValidateTransactionScripts(tx, blockUtxos,
			txscript.StandardVerifyFlags, g.sigCache,
			g.hashCache)
		if err != nil {
			log.Tracef("skipping tx %s due to error in "+
				"validatetransactionscripts:%v", tx.Hash(), err)
			logSkippedDeps(tx, deps)
			continue
		}

		// Spend the transaction inputs in the block utxo view and add
		// an entry for it to ensure any transactions which reference
		// this one have it available as an input and can ensure they
		// aren't double spending.
		spendTransaction(blockUtxos, tx, nextBlockHeight)

		//add the transaction to the block ,increment counters.and
		//save the fees and signature operation counts to the block
		//template.
		blockTxns = append(blockTxns, tx)
		blockWeight += txWeight
		blockSigOpCost += int64(sigOpCost)
		totalFees += prioItem.fee
		txFees = append(txFees, prioItem.fee)
		txSigOpCosts = append(txSigOpCosts, int64(sigOpCost))

		log.Tracef("adding tx %s (priority %.2f,feePerKb %.2f)", prioItem.tx.Hash(), prioItem.priority, prioItem.feePerKB)

		//add transaction which depend on this one (and also do not have any other
		//unsatisifed dependcies ) to the priority queue
		for _, item := range deps {
			//add the transaction to the priority queue if there
			//are no more dependencies after this one.
			delete(item.dependsOn, *tx.Hash())
			if len(item.dependsOn) == 0 {
				heap.Push(priorityQueue, item)
			}
		}

	}

	//now that the actual transaction have been selected .update the block
	//weitht for the real transaction count and coinbase value with the
	//total fees accrordingly.
	blockWeight -= wire.MaxVarIntPayload - (uint32(wire.VarIntSerializeSize(uint64(len(blockTxns)))) *
		blockchain.WitnessScaleFactor)

	coinbaseTx.MsgTx().TxOut[0].Value += totalFees
	txFees[0] = -totalFees

	//if segwit is active and we included transaction with witness data.
	//then we will need to include a commmitment to the witness data in an
	//op_return about within the coinbase transaction.
	var witnessCommitment []byte
	if witnessIncluded {
		//the witness of coinbase transaction must be exactly 32-bytes of all zerol
		var witnessNonce [blockchain.CoinbaseWitnessDataLen]byte
		coinbaseTx.MsgTx().TxIn[0].Witness = wire.TxWitness{witnessNonce[:]}

		//next.obtain the merkle root of a tree which consists of the wtxid of
		//all transaction in the block ,the coinbase transaction will have a
		//s special wtxid of all zero
		witnessMerkleTree := blockchain.BuildMerkleTreeStore(blockTxns,
			true)
		witnessMerkleRoot := witnessMerkleTree[len(witnessMerkleTree)-1]

		//the preimage to the witness commitment is:
		//witnessRoot || coinbaseWitness.
		var witnessPreimage [64]byte
		copy(witnessPreimage[:32], witnessMerkleRoot[:])
		copy(witnessPreimage[32:], witnessNonce[:])

		//the witness commitness iteself is the double-sha256 of the witness
		//preimage generated above.with the commitment generated.the witness
		//script for the output is :op_return op_data_36{0xaa21a9ed || witnessCommitment}. The leading
		// prefix is referred to as the "witness magic bytes".
		witnessCommitment = chainhash.DoubleHashB(witnessPreimage[:])
		witnessScript := append(blockchain.WitnessMagicBytes, witnessCommitment...)

		//finally create the op_return carrying witness commitnment output
		//as an addition output within the coinbase
		commitmentOutput := &wrie.TxOut{
			Value:    0,
			PkScript: witnessScript,
		}
		coinbaseTx.MsgTx().TxOut = append(coinbaseTx.MsgTx().TxOut,
			commitmentOutput)

	}

	//calculate the required difficuly for the block .the timestamp is potentiasly
	//adjust to ensure it comes after the median time of the last sevenral blcoks
	//per the chain consensus rules.

	ts := medianAdjustTime(best, g.timeSource)
	reqDifficult, err := g.chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		return nil, err
	}

	//calclulate the next expected block version based on the state of the rule
	//change deployment.
	nextBlockVersion, err := g.chain.CalcNextBlockVersion()
	if err != nil {
		return nil, err
	}

	//create a new block ready to be solved
	merkles := blockchain.BuildMerkleTreeStore(blockTxns, false)
	var msgBlock wire.MsgBlock
	msgBlock.Header = wire.BlockHeader{
		Version:    nextBlockHeight,
		PrevBlock:  best.Hash,
		MerkleRoot: *merkles[len(merkles)-1],
		Timestamp:  ts,
		Bits:       reqDifficult,
	}

	for _, tx := range blockTxns {
		if err := msgBlock.AddTransaction(tx.MsgTx()); err != nil {
			return nil, err
		}
	}

	//finanly ,perform a full check on the created block against the chain
	//consensus rules to ensure it properly conncets to the current best
	//chain with no isssues.
	block := btcutil.NewBlock(&msgBlock)
	block.SetHeight(nextBlockHeight)
	if err := g.chain.CheckConnectBlockTemplate(block); err != nil {
		return nil, err
	}

	log.Debugf("Created new block template (%d transactions, %d in "+
		"fees, %d signature operations cost, %d weight, target difficulty "+
		"%064x)", len(msgBlock.Transactions), totalFees, blockSigOpCost,
		blockWeight, blockchain.CompactToBig(msgBlock.Header.Bits))

	return &BlockTemplate{
		Block:             &msgBlock,
		Fees:              txFees,
		SigOpCost:         txSigOpCosts,
		Height:            nextBlockHeight,
		ValidPayAddress:   payToAddress != nil,
		WitnessCommitment: witnessCommitment,
	}, nil

}

//updateblocktime updates the timestamp in the header of the
//passed block to the current time while taking into account the chain
//consensus rules .finanal ,it will update the target difficult if need
//based on the new time for the test network since their target difficult can
//change based upon time.
func (g *BlkTmplGenerator) UpdateBlockTime(msgBlock *wire.MsgBlock) error {
	//the new timestamp is potentially adjusted to ensure it comes after
	//the median time of the last several blcoks per the chain consensus rules.
	newTime := medianAdjustTime(g.chain.BestSnapshot(), g.timeSource)
	msgBlock.Header.Timestamp = newTime

	//recalulate the difficulty if running on a network that requires it .
	if g.chainParams.ReduceMinDifficulty {
		difficulty, err := g.chain.CalcNextRequiredDifficulty(newTime)
		if err != nil {
			return err
		}
		msgBlock.Header.Bits = difficulty
	}

	return nil

}

//updatextranonce updates teh extra nonce in the coinbase script of the
//passed block by regenearting the coinbase script the passed value
//and block hegith .it also recalulates and updates the new merkele
//root that results form changing the coinbase script
func (g *BlkTmplGenerator) UpdateExtraNonce(msgBlock *wire.MsgBlock, blockHeight int32, extraNonce uint64) error {
	coinbaseScript, err := standardCoinbaseScript(blockHeight, extraNonce)
	if err != nil {
		return err
	}

	if len(coinbaseScript) > blockchain.MaxCoinbaseScriptLen {
		return fmt.Errorf("coinbase transaction script lenth"+
			"of %d is out of range (min:%d,max:%d)", len(coinbaseScript), blockchain.MinCoinbaseScriptLen,
			blockchain.MaxCoinbaseScriptLen)
	}

	msgBlock.Transactions[0].TxIn[0].SignatureScript = coinbaseScript

	//recalculate the merkle root with the updated extra nonce.
	block := btcutil.NewBlock(msgBlock)
	merkles := blockchain.BuildMerkleTreeStore(block.Transactions(), false)
	msgBlock.Header.MerkleRoot = *merkles[len(merkles)-1]

	return nil

}

// BestSnapshot returns information about the current best chain block and
// related state as of the current point in time using the chain instance
// associated with the block template generator.  The returned state must be
// treated as immutable since it is shared by all callers.
//
// This function is safe for concurrent access.
func (g *BlkTmplGenerator) BestSnapshot() *blockchain.BestState {
	return g.chain.BestSnapshot()
}

// TxSource returns the associated transaction source.
//
// This function is safe for concurrent access.
func (g *BlkTmplGenerator) TxSource() TxSource {
	return g.txSource
}

//over























