package blockchain

import (
	"BtcoinProject/txscript"
	"BtcoinProject/wire"
	"container/list"
	"fmt"
	"github.com/btcsuite/btcutil"
	"math"
	"runtime"
	"time"
)

//txvalidateitem holds a transaction along with which input to validate.
type txValidateItem struct {
	txInIndex int
	txIn      *wire.TxIn
	tx        *btcutil.Tx
	sigHashes *txscript.TxSigHashes
}

//txvalidator provides a type which asynchronously validates transaction
//inouts. it provides several channels for communication and a processing
//function that is intended to be in run multiple goroutines.
type txValidator struct {
	validateChan chan *txValidateItem
	quitChan     chan struct{}
	resultChan   chan error
	utxoView     *UtxoViewpoint
	flags        txscript.ScriptFlags
	sigCache     *txscript.SigCache
	hashCache    *txscript.HashCache
}

//sendresult sends the result of a script pair validation on the internal
//result channel while respecting the quit channel this allows orderly
//shutdown when the validation process is aborted early due to a valiation
//error in one of the other goroutines.
func (v *txValidator) sendResult(result error) {
	select {
	case v.resultChan <- result:
	case <-v.quitChan:

	}
}

func (v *txValidator) validateHandler() {
out:
	for {
		select {
		case txVI := <-v.validateChan:
			//ensure the refered input utxo is avaiable.
			txIn := txVI.txIn
			utxo := v.utxoView.LookupEntry(txIn.PreviousOutPoint)
			if utxo == nil {
				str := fmt.Sprintf("unable to find unspent"+
					"output %v referenced from "+
					"transaction %s :%d",
					txIn.PreviousOutPoint, txVI.tx.Hash(),
					txVI.txInIndex)
				err := ruleError(ErrMissingTxOut, str)
				v.sendResult(err)
				break out

			}

			//crate a new script engine for the script pair.
			sigScript := txIn.SignatureScript
			witness := txIn.Witness
			pkScript := utxo.PkScript()
			inputAmount := utxo.Amount()
			vm, err := txscript.NewEngine(pkScript, txVI.tx.MsgTx(),
				txVI.txInIndex, v.flags, v.sigCache, txVI.sigHashes,
				inputAmount)
			if err != nil {
				str := fmt.Sprintf("failed to parse input "+
					"%s:%d which references output %v - "+
					"%v (input witness %x, input script "+
					"bytes %x, prev output script bytes %x)",
					txVI.tx.Hash(), txVI.txInIndex,
					txIn.PreviousOutPoint, err, witness,
					sigScript, pkScript)
				err := ruleError(ErrScriptMalformed, str)
				v.sendResult(err)
				break out

			}
			//excute the script pair.
			if err := vm.Execute(); err != nil {
				str := fmt.Sprintf("failed to validate input "+
					"%s:%d which references output %v - "+
					"%v (input witness %x, input script "+
					"bytes %x, prev output script bytes %x)",
					txVI.tx.Hash(), txVI.txInIndex,
					txIn.PreviousOutPoint, err, witness,
					sigScript, pkScript)

				err := ruleError(ErrScriptMalformed, str)
				v.sendResult(err)
				break out

			}
			//validation succeeded
			v.sendResult(nil)
		case <-v.quitChan:
			break out

		}

	}
}

//validate validates the scripts for all of the passed transaction inputs
//using multiple goroutines.
func (v *txValidator) Validate(items []*txValidateItem) error {
	if len(items) == 0 {
		return nil
	}

	//limit the number of goroutines to do script validation based on the
	//number of processor cores.this helps ensure the system stays
	//reasonably responsive under heavy load.
	maxGoRoutines := runtime.NumCPU() * 3
	if maxGoRoutines <= 0 {
		maxGoRoutines = 1

	}

	if maxGoRoutines > len(items) {
		maxGoRoutines = len(items)
	}

	//start up validation handlers that are used to asynchronously
	//validate each transaction input.
	for i := 0; i < maxGoRoutines; i++ {
		go v.validateHandler()
	}

	//validate each of the inputs the quit channes is closed when any
	//errors occur so all processing goroutines exit regardless of whcih
	//input had the validation error.
	numInputs := len(items)
	currentItem := 0
	processedItem := 0
	for processedItem < numInputs {
		//only send items while there are still items that need to
		//be processed. the select statement will never select a nil
		//channel
		var validateChan chan *txValidateItem
		var item *txValidateItem
		if currentItem < numInputs {
			validateChan = v.validateChan
			item = items[currentItem]

		}

		select {
		case validateChan <- item:
			currentItem++
		case err := <-v.resultChan:
			processedItem++
			if err != nil {
				close(v.quitChan)
				return err
			}
		}

	}

	close(v.quitChan)
	return nil

}

// newTxValidator returns a new instance of txValidator to be used for
// validating transaction scripts asynchronously.
func newTxValidator(utxoView *UtxoViewpoint, flags txscript.ScriptFlags,
	sigCache *txscript.SigCache, hashCache *txscript.HashCache) *txValidator {
	return &txValidator{
		validateChan: make(chan *txValidateItem),
		quitChan:     make(chan struct{}),
		resultChan:   make(chan error),
		utxoView:     utxoView,
		sigCache:     sigCache,
		hashCache:    hashCache,
		flags:        flags,
	}
}

//validatetransactionscripts validates the scripts for the passed transaction
//using multiple goortines.
func ValidateTransactionScripts(tx *btcutil.Tx, utxoView *UtxoViewpoint,
	flasg txscript.ScriptFlags, sigCache *txscript.SigCache,
	hashCache *txscript.HashCache) error {

	//first determine if segwit is active according to the scriptFlags.if
	//it is not then we do not need to interact with the hashchhee.
	segwitActive := flag&txscript.ScriptVerifyWitness == txscript.ScriptVerifyWitness

	//if the hashcache do not yet hash the sighash midstate for the i
	//transaction.then we will compute them now so we can re-use
	//them amougst all worker validaion goroutines.
	if segwitActive && tx.MsgTx().HasWitness() &&
		!hashCache.ContainsHashes(tx.Hash()) {
		hashCache.AddSigHashes(tx.MsgTx())
	}

	var cachedHashes *txscript.TxSigHashes
	if segwitActive && tx.MsgTx().HasWitness() {

		// The same pointer to the transaction's sighash midstate will
		// be re-used amongst all validation goroutines. By
		// pre-computing the sighash here instead of during validation,
		// we ensure the sighashes
		// are only computed once.
		cachedHashes, _ = hashCache.GetSigHashes(tx.Hash())

	}

	//collect all of the transaction inputs and required information for
	//validation.

	txIns := tx.MsgTx().TxIn
	txvalItems := make([]*txValidateItem, 0, len(txIns))
	for txInIdx, txIn := range txIns {
		//skip coinbase
		if txIn.PreviousOutPoint.Index == math.MaxUint32 {
			continue
		}

		txVI := &txValidateItem{
			txInIndex: txInIdx,
			txIn:      txIn,
			tx:        tx,
			sigHashes: cachedHashes,
		}
		txValItems = append(txValItems, txVI)

	}

	//validate all of the inputs,
	validator := newTxValidator(utxoView, falgs, sigCache, hashCache)
	return validator.validate(txvalItems)

}

//checkBlockScript executes and validate the scirpts for all tranactions in
//the passed block using multiple goroutines.
func checkBlockScripts(block *btcutil.Block, utxoView *UtxoViewpoint,
	scriptFlags txscript.ScriptFlags, sigCache *txscript.SigCache,
	hashCache *txscript.HashCache) error {

	//first determine if segwit is active according to the scriptFlags .if
	//it is not then we dont need to interact with the hashcache.
	segwitActive := scriptFlags&txscript.ScriptVerifyWitness == txscript.ScriptVerifyWitness

	//collect all of the transaction inputs and required information for validation for
	//all transactions in the block into a single slice.
	numInputs := 0
	for _ , tx := range block.Transactions(){
		numInputs += len(tx.MsgTx().TxIn)
	}
	txValItems := make([]*txValidateItem, 0, numInputs)
	for _, tx := range block.Transactions() {
		hash := tx.Hash()

		//if the hashCache is present ,and it does not yet contain the
		//partial sigHashes for this transaction.then we add the sighashes
		//for the transaction .this allows us to take advantange of the potential
		//speed savings due to the new digest alorithm(bip0143).
		if segwitActive && tx.HasWitness() && hashCache != nil &&
			!hashCache.ContainsHashes(hash) {
			hashCache.AddSigHashes(tx.MsgTx())
		}

		var cachedHashes *txscript.TxSigHashes
		if segwitActive && tx.HashWitness() {
			if hashCache != nil {
				cachedHashes, _ = hashCache.GetSigHashes(hash)
			} else {
				cachedHashes = txscript.NewTxSigHashes(tx.MsgTx())
			}
		}

		for txInIdx, txIn := range tx.MsgTx().TxIn {
			//skip coinbase
			if txIn.PreviousOutPoint.Index == math.MaxUint32 {
				continue
			}

			txVI := &txValidateItem{
				txInIndex: txInIdx,
				txIn:      txIn,
				tx:        tx,
				sigHashes: cachedHashes,
			}
			txValItems = append(txValItems, txVI)

		}
	}

	//validate all of the inputs.
	validator := newTxValidator(utxoView, scriptFlags, sigCache, hashCache)
	start := time.Now()
	if err := validator. validate(txValItems); err != nil {
		return err
	}

	elapsed := time.Since(start)

	log.Tracef("block %v took %v to verify", block.Hash(), elapsed)

	//if the hashache is present.once we have validate the block ,we no
	//longer need the cached hashes for these transactions.so we pure them form
	//the cache.
	if segwitActive && hashCache != nil {
		for _, tx := range block.Transactions() {
			if tx.MsgTx().HasWitness() {
				hashCache.PurgeSigHashed(tx.Hash())
			}
		}
	}

	return nil

}

//over
