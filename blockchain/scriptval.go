package blockchain

import (
	"BtcoinProject/wire"
	"github.com/btcsuite/btcutil"
)

//txvalidateitem holds a transaction along with which input to validate.
type txValidateItem struct {
	txInIndex int
	txIn *wire.TxIn
	tx *btcutil.Tx
	sigHashes *txscript.TxSigHashes
}

