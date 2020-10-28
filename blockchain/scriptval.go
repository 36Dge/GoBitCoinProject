package blockchain

import (
	"BtcoinProject/wire"
	"container/list"
	"fmt"
	"github.com/btcsuite/btcutil"
	"runtime"
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

//validate


























