package mempool

import (
	"BtcoinProject/chaincfg/chainhash"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/btcsuite/btcutil"
	"io"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
)

const (

	//estimatefeedepth is the maximum number of blocks before a transaction
	//is confirmed that we want to track
	estimateFeeDepth = 25

	//estimateFeebinsize is the number of txs stored in each bin.
	estimateFeeBinSize = 100

	//estimatefeemaxreplacements is the max number of replacements that
	//can be made by the txs found in a given block.
	estimateFeeMaxReplacements = 10

	//defaultestimatefeemaxrollback is the default number of rollbacks
	//allowed by the fee estimator for orpahaned blocks.
	DefaultEstimateFeeMaxRollback = 2

	//defaultestimatefeeminregisteredblocks is the default minimum
	//number of blocks which must be observed by the estimator before
	//it will provide fee estimations.
	DefaultEstimateFeeMinRegisteredBlocks = 3

	bytePerKb = 1000

	btcPerSatoshi = 1e-8
)

var (
	//estimatefeedatabasekey is the key that we use to store
	//the fee estimator in the database
	EstimateFeeDatabaseKey = []byte("estimatefee")
)

//satoshiperbyte in number with units of satoshis per byte
type SatoshiPerByte float64

//Btcperkilobyte is number with uints of bitcoins per kilobyte
type BtcPerKilobyte float64

//tobtcperkb returns a float value that reperesent the given
//satoshiperbyte converted to satoshis per kb
func (rate SatoshiPerByte) ToBtcPerKb() BtcPerKilobyte {
	//if our rate is the error value ,return that
	if rate == SatoshiPerByte(-1.0) {
		return -1.0
	}
	return BtcPerKilobyte(float64(rate) * bytePerKb * btcPerSatoshi)
}

//fee returns the fee for a  transaction of a given size for the given fee rate
func (rate SatoshiPerByte) Fee(size uint32) btcutil.Amount {
	//if our rate is the error value ,return that
	if rate == SatoshiPerByte(-1.0) {
		return btcutil.Amount(-1)
	}
	return btcutil.Amount(float64(rate) * float64(size))
}

//newsatoshiperbyte creates a satoshiperbyte from an amount and a size in byte
func NewSatoshiPerByte(fee btcutil.Amount, size uint32) SatoshiPerByte {
	return SatoshiPerByte(float64(fee) / float64(size))
}

//observedTransatcion repreensent an observed transaction and some
//additional data required for the fee estimation algorithm
type observedTransaction struct {

	//a tranaction hash
	hash chainhash.Hash

	//the fee per byte of transaction in satoshi
	feeRate SatoshiPerByte

	//the block height when it was observed
	observed int32

	//the height of block in which it was mined
	//if the transaction has not yet been mined. it is zero
	mined int32
}

func (o *observedTransaction) Serialize(w io.Writer) {
	binary.Write(w, binary.BigEndian, o.hash)
	binary.Write(w, binary.BigEndian, o.feeRate)
	binary.Write(w, binary.BigEndian, o.observed)
	binary.Write(w, binary.BigEndian, o.mined)
}

func deserializeObservedTransaction(r io.Reader) (*observedTransaction, error) {

	ot := observedTransaction{}

	//the first 32 bytes should be a hash
	binary.Read(r, binary.BigEndian, &ot.hash)

	//the next 8 are satoshiperbyte
	binary.Read(r, binary.BigEndian, &ot.feeRate)

	//and next there are two uint32,s.
	binary.Read(r, binary.BigEndian, &ot.observed)
	binary.Read(r, binary.BigEndian, &ot.mined)

	return &ot, nil
}

//registeredblock has the hash of a block and the list of transactions it mined
//which had been previously observed by the feeestimator .it is used if rollback
//is called to reserve the effect of registering a block
type registeredBlock struct {
	hash         chainhash.Hash
	transactions []*observedTransaction
}

func (rb *registeredBlock) serialize(w io.Writer, txs map[*observedTransaction]uint32) {
	binary.Write(w, binary.BigEndian, rb.hash)
	binary.Write(w, binary.BigEndian, uint32(len(rb.transactions)))
	for _, o := range rb.transactions {
		binary.Write(w, binary.BigEndian, txs[o])
	}
}

//feeestimator manages the data necessary to create
//fee estimations .it it safe for concurrent access

type FeeEstimator struct {
	maxRollback uint32
	binSize     int32

	//the maximum number of replacement that can be made in a single
	//bin per block .default is estimateFeemaxreplacements
	maxReplacements int32

	//the minimum number of blocks that can be registered with the fee
	//estimator before it will provide answers.
	minRegisteredBlcoks uint32

	//the last known height
	lastKnownHeight int32

	//the number of blocks that have been registered.
	numBlocksRegistered uint32

	mtx sync.RWMutex

	observed map[chainhash.Hash]*observedTransaction
	bin      [estimateFeeDepth][]*observedTransaction

	//the cached estimate
	cached []SatoshiPerByte

	//transactions that have been removed from the bins .this allowd us to
	//revert in case of an orphaned block.
	dropped []*registeredBlock
}

//newfeeestimator creates a feeestimator for which at most maxrollback blocks
//can be unregistered and which returns an error unless minRegisteredBlcok
//have been registered with it
func NewFeeEstimator(maxRollback, minRegisteredBlcoks uint32) *FeeEstimator {
	return &FeeEstimator{
		maxRollback:         maxRollback,
		binSize:             estimateFeeBinSize,
		maxReplacements:     estimateFeeMaxReplacements,
		minRegisteredBlcoks: minRegisteredBlcoks,
		lastKnownHeight:     mining.UnminedHeight,
		observed:            make(map[chainhash.Hash]*observedTransaction),
		dropped:             make([]*registeredBlock, 0, maxRollback),
	}
}

//observedtrancaction is called when a new transaction is observed in the
//mempool
func (ef *FeeEstimator) ObserveTransaction(t *TxDesc) {
	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	//if we haven,t seen a block yet we don,t know when this one arrived
	//so we ignore it.
	if ef.lastKnownHeight == mining.UnminedHeight {
		return
	}

	hash := *t.Tx.Hash()
	if _, ok := ef.observed[hash]; !ok {
		size := uint32(GetTxVirtualSize(t.Tx))

		ef.observed[hash] = &observedTransaction{
			hash:     hash,
			feeRate:  NewSatoshiPerByte(btcutil.Amount(t.Fee), size),
			observed: t.Height,
			mined:    mining.UnminedHeight,
		}

	}

}

//registerBlock informs the fee estimator of a new block to take into account
func (ef *FeeEstimator) RegisterBlcok(block *btcutil.Block) error {
	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	//the previous sorted list is invalid ,so delete it.
	ef.cached = nil

	height := block.Height()
	if height != ef.lastKnownHeight+1 && ef.lastKnownHeight != mining.UnminedHeight {
		return fmt.Errorf("intermediate block not recorded;current heigth is %d ;new height is %d",
			ef.lastKnownHeight, height)
	}

	//update the last known height
	ef.lastKnownHeight = height
	ef.numBlocksRegistered++

	//reandomly order txs in block.
	transactions := make(map[*btcutil.Tx]struct{})
	for _, t := range block.Transactions() {
		transactions[t] = struct{}{}

	}

	//count the number of replacements we make per bin so that we dont
	//replace too many
	var replacementCounts [estimateFeeDepth]int

	//keep track of which txs were doropped in case of an orphan block

	dropped := &registeredBlock{
		hash:         *block.Hash(),
		transactions: make([]*observedTransaction, 0, 100),
	}

	// go through the txs in the block.

	for t := range transactions {
		hash := *t.Hash()

		//have we observed this tx in the mempool?
		o, ok := ef.observed[hash]
		if !ok {
			continue
		}

		//put the observed tx in the oppropritate bin.
		blockToConfirm := height - o.observed - 1

		//this shouldn,t happen if the fee estimator works correnctly
		//but return an error if it does.
		if o.mined != mining.UnminedHeight {
			log.Error("Estimate fee: transaction ", hash.String(), " has already been mined")
			return errors.New("transaction has already been mined")
		}

		// this should,t happen but check just in case to avoid
		// an out_of_bounds array index later.
		if blockToConfirm >= estimateFeeDepth {
			continue
		}

		//make sure we dont replace too many transactions per min.

		if replacementCounts[blockToConfirm] == int(ef.maxReplacements) {
			continue
		}

		o.mined = height
		replacementCounts[blockToConfirm]++

		bin := ef.bin[blockToConfirm]

		//remove a random element and replace it with this new tx
		if len(bin) == int(ef.binSize) {
			//don not drop transactions we have just added this same block.
			l := int(ef.binSize) - replacementCounts[blockToConfirm]
			drop := rand.Intn(l)
			dropped.transactions = append(dropped.transactions.bin[drop])
			bin[drop] = bin[l-1]
			bin[l-1] = o

		} else {
			bin = append(bin, o)
		}

		ef.bin[blockToConfirm] = bin

	}

	// go through the mempool for txs that have been in too long.
	for hash, o := range ef.observed {
		if o.mined == mining.UnminedHeight && height-o.observed >= estimateFeeDepth {
			delete(ef.observed, hash)
		}
	}

	//add dropped list to history
	if ef.maxRollback == 0 {
		return nil
	}

	if uint32(len(ef.dropped)) == ef.maxRollback {
		ef.dropped = append(ef.dropped[1:], dropped)
	} else {
		ef.dropped = append(ef.dropped, dropped)
	}

	return nil

}

//lastknownheight returns the height of the last blcok which was registed
func (ef *FeeEstimator) LastKnownHeight() int32 {
	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	return ef.lastKnownHeight
}

// Rollback unregisters a recently registered block from the FeeEstimator.
// This can be used to reverse the effect of an orphaned block on the fee
// estimator. The maximum number of rollbacks allowed is given by
// maxRollbacks.
//
// Note: not everything can be rolled back because some transactions are
// deleted if they have been observed too long ago. That means the result
// of Rollback won't always be exactly the same as if the last block had not
// happened, but it should be close enough.

func (ef *FeeEstimator) Rollback(hash *chainhash.Hash) error {
	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	//find this block in the stacks of recent registered blocks.
	var n int
	for n = 1; n <= len(ef.dropped); n++ {
		if ef.dropped[len(ef.dropped)-n].hash.IsEqual(hash) {
			break
		}
	}

	if n > len(ef.dropped) {
		return errors.New("no such block was receently registered")
	}

	for i := 0; i < n; i++ {
		ef.rollback()
	}

	return nil

}

//rollback rolls back the effect of the last blcok in the stack of registered blcoks
func (ef *FeeEstimator) rollback() {
	//the previous sorted list is invalid,so delete it.
	ef.cached = nil

	//pop the last list of dropped txs from the stack
	last := len(ef.dropped) - 1
	if last == -1 {
		//cannot really happen because the exported calling function
		//only rolls back a block already known to be in the list of
		//dropped trasncaction
		return

	}

	dropped := ef.dropped[last]

	//where we are in each bin was we replace txs?

	var replacementCounters [estimateFeeDepth]int

	//go throught the txs in the dropped block.
	for _, o := range dropped.transactions {
		//which bin was this tx in?
		blocksToConfirm := o.mined - o.observed - 1

		bin := ef.bin[blocksToConfirm]

		var counter = replacementCounters[blocksToConfirm]

		//continue to go through that bin where we left off.
		for {
			if counter >= len(bin) {
				//panic
				panic(errors.New("illegal state :cannot rollback dropped transaction"))
			}

			prev := bin[counter]

			if prev.mined == ef.lastKnownHeight {
				prev.mined = mining.UnminedHeight

				bin[counter] = o
				counter++
				break

			}

			counter++

		}
		replacementCounters[blocksToConfirm] = counter
	}
	//continue going through bins to find others txs to remove
	//which did not replace anyt other when they were entered
	for i, j := range replacementCounters {
		for {
			l := len(ef.bin[i])
			if j >= l {
				break
			}

			prev := ef.bin[i][j]

			if prev.mined == ef.lastKnownHeight {
				prev.mined = mining.UnminedHeight

				newBin := append(ef.bin[i][0:j], ef.bin[i][j+1:l]...)
				// TODO This line should prevent an unintentional memory
				// leak but it causes a panic when it is uncommented.
				// ef.bin[i][j] = nil
				ef.bin[i] = newBin
				continue
			}
			j++
		}

	}

	ef.dropped = ef.dropped[0:last]

	//the number of blocks the fee estimator has seen is decrimented.
	ef.numBlocksRegistered--
	ef.lastKnownHeight--

}

//estimateFeeSet is a set of txs that can that is sorted
//by the fee per kb rate.
type estimateFeeSet struct {
	feeRate []SatoshiPerByte
	bin     [estimateFeeDepth]uint32
}

func (b *estimateFeeSet) Len() int {
	return len(b.feeRate)
}

func (b *estimateFeeSet) Less(i, j int) bool {
	return b.feeRate[i] > b.feeRate[j]
}

func (b *estimateFeeSet) Swap(i, j int) {
	b.feeRate[i], b.feeRate[j] = b.feeRate[j], b.feeRate[i]
}

//estimatefee returns the estimated fee for a transaction to
//confirm in confirmations blocks from now,given the data set
//we have collected.
func (b *estimateFeeSet) estimateFee(confirmations int) SatoshiPerByte {
	if confirmations <= 0 {
		return SatoshiPerByte(math.Inf(1))

	}

	if confirmations > estimateFeeDepth {
		return 0

	}

	//we don,t have any trancactions!
	if len(b.feeRate) == 0 {
		return 0
	}

	var min, max int = 0, 0
	for i := 0; i < confirmations-1; i++ {
		min += int(b.bin[i])
	}

	max = min + int(b.bin[confirmations-1]) - 1
	if max < min {
		max = min
	}

	feeIndex := (max + min) / 2
	if feeIndex >= len(b.feeRate) {
		feeIndex = len(b.feeRate) - 1
	}

	return b.feeRate[feeIndex]

}

//new estimatefeeset creates a temporary data structure that
//can be used to find all fee estimates.
func (ef *FeeEstimator) newEstimateFeeSet() *estimateFeeSet {
	set := &estimateFeeSet{}

	capacity := 0
	for i, b := range ef.bin {
		l := len(b)
		set.bin[i] = uint32(l)
		capacity += l
	}

	set.feeRate = make([]SatoshiPerByte, capacity)

	i := 0
	for _, b := range ef.bin {
		for _, o := range b {
			set.feeRate[i] = o.feeRate
		}
		i++
	}
	sort.Sort(set)

	return set
}

//estimates returns the set of all fee estimates from 1 to estimatefeedepth
//confirmations from now.
func (ef *FeeEstimator) estimates() []SatoshiPerByte {
	set := ef.newEstimateFeeSet()

	estimates := make([]SatoshiPerByte, estimateFeeDepth)
	for i := 0; i < estimateFeeDepth; i++ {
		estimates[i] = set.estimateFee(i + 1)
	}

	return estimates

}

//estimatefee estimates the fee per byte to have a tx confirmed a
//given number of blocks from mow.
func (ef FeeEstimator) EstimateFee(numBlocks uint32) (BtcPerKilobyte, error) {
	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	//if the number of registered blocks in below the minimum ,return an error
	if ef.numBlocksRegistered < ef.minRegisteredBlcoks {
		return -1, errors.New("not enough blcoks have been observed")
	}

	if numBlocks == 0 {
		return -1, errors.New("cannot confirm transaction in zero blocks")
	}

	if numBlocks > estimateFeeDepth {
		return -1, fmt.Errorf(
			"can only estimate fees for up to %d blocks from now",
			estimateFeeBinSize)
	}
	//if there are no cached results .generate them

	if ef.cached == nil {
		ef.cached = ef.estimates()
	}
	return ef.cached[int(numBlocks)-1].ToBtcPerKb(), nil

}

// in case the format for the serialized version of the feeestimator changes
//we use a version number.if the version number changes.it does not make sence
//to try to upgrade a previous version to a new version.instead.just started fee
//estimation over.

const estimateFeeSaveVersion = 1

func deserializeRegisteredBlcok(r io.Reader, txs map[uint32]*observedTransaction) (*registeredBlock, error) {

	var lenTransactions uint32

	rb := &registeredBlock{}
	binary.Read(r, binary.BigEndian, &rb.hash)
	binary.Read(r, binary.BigEndian, &lenTransactions)

	rb.transactions = make([]*observedTransaction, lenTransactions)

	for i := uint32(0); i < lenTransactions; i++ {
		var index uint32
		binary.Read(r, binary.BigEndian, &index)
		rb.transactions[i] = txs[index]
	}
	return rb, nil

}

//feeestimateorstate represents a saved feeestimator that can be restored
//with data from an earlier session of the program.
type FeeEstimatorState []byte

//observedTxSet is a set of txs that can that is sorted by hash.
//it exists for serialization purposes so that a serialized state
//always comes out the same.
type observedTxSet []*observedTransaction

func (q observedTxSet) Len() int {
	return len(q)
}

func (q observedTxSet) Less(i, j int) bool {
	return strings.Compare(q[i].hash.String(), q[j].hash.String()) < 0
}

func (q observedTxSet) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

//save records the current state of the feeestimator to a []byte that
//can be restored later

func (ef *FeeEstimator) Save() FeeEstimatorState {

	ef.mtx.Lock()
	defer ef.mtx.Unlock()

	//todo
	w := bytes.NewBuffer(make([]byte, 0))

	binary.Write(w, binary.BigEndian, uint32(estimateFeeSaveVersion))

	//insert basic parameters.

	binary.Write(w, binary.BigEndian, &ef.maxRollback)
	binary.Write(w, binary.BigEndian, &ef.binSize)
	binary.Write(w, binary.BigEndian, &ef.maxReplacements)
	binary.Write(w, binary.BigEndian, &ef.minRegisteredBlcoks)
	binary.Write(w, binary.BigEndian, &ef.lastKnownHeight)
	binary.Write(w, binary.BigEndian, &ef.numBlocksRegistered)

	//put all the observed transactions in a sorted list.
	var txCount uint32

	ots := make([]*observedTransaction, len(ef.observed))
	for hash := range ef.observed {
		ots[txCount] = ef.observed[hash]
		txCount++
	}

	sort.Sort(observedTxSet(ots))

	txCount = 0
	observed := make(map[*observedTransaction]uint32)
	binary.Write(w, binary.BigEndian, uint32(len(ef.observed)))
	for _, ot := range ots {
		ot.Serialize(w)
		observed[ot] = txCount
		txCount++
	}

	//save all the right bins
	for _, list := range ef.bin {

		binary.Write(w, binary.BigEndian, uint32(len(list)))

		for _, o := range list {
			binary.Write(w, binary.BigEndian, observed[o])
		}
	}

	//dropped transactions
	binary.Write(w, binary.BigEndian, uint32(len(ef.dropped)))
	for _, registered := range ef.dropped {
		registered.serialize(w, observed)
	}

	//commit the tx and return
	return FeeEstimatorState(w.Bytes())

}

//restorefeeestimator takes a feeestimatorstate that was previously
//retured by save and restores it to a feeestimator

func RestoreFeeEstimator(data FeeEstimatorState) (*FeeEstimator, error) {
	r := bytes.NewReader(data)

	//check version
	var version uint32
	err := binary.Read(r, binary.BigEndian, &version)
	if err != nil {
		return nil, err
	}
	if version != estimateFeeSaveVersion {
		return nil, fmt.Errorf("Incorrect version: expected %d found %d", estimateFeeSaveVersion, version)
	}

	ef := &FeeEstimator{
		observed: make(map[chainhash.Hash]*observedTransaction),
	}

	//read basic parameters

	binary.Read(r, binary.BigEndian, &ef.maxRollback)
	binary.Read(r, binary.BigEndian, &ef.binSize)
	binary.Read(r, binary.BigEndian, &ef.maxReplacements)
	binary.Read(r, binary.BigEndian, &ef.minRegisteredBlcoks)
	binary.Read(r, binary.BigEndian, &ef.lastKnownHeight)
	binary.Read(r, binary.BigEndian, &ef.numBlocksRegistered)

	//read transactions.
	var numObserved uint32
	observed := make(map[uint32]*observedTransaction)
	binary.Read(r, binary.BigEndian, &numObserved)
	for i := uint32(0); i < numObserved; i++ {
		ot, err := deserializeObservedTransaction(r)
		if err != nil {
			return nil, err
		}
		observed[i] = ot
		ef.observed[ot.hash] = ot
	}

	//read bins
	for i := 0; i < estimateFeeDepth; i++ {
		var numTransactions uint32
		binary.Read(r, binary.BigEndian, &numTransactions)
		bin := make([]*observedTransaction, numTransactions)
		for j := uint32(0); j < numTransactions; j++ {
			var index uint32
			binary.Read(r, binary.BigEndian, &index)

			var exists bool
			bin[j], exists = observed[index]
			if !exists {
				return nil, fmt.Errorf("invalid transaction reference %d", index)
			}

		}
		ef.bin[i] = bin
	}

	// read dropped transactions

	var numDropped uint32
	binary.Read(r, binary.BigEndian, &numDropped)
	ef.dropped = make([]*registeredBlock, numDropped)
	for i := uint32(0); i < numDropped; i++ {
		var err error
		ef.dropped[int(i)], err = deserializeRegisteredBlcok(r, observed)
		if err != nil {
			return nil, err
		}
	}

	return ef, nil
}

//over