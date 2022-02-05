package txscript

import "BtcoinProject/chaincfg/chainhash"

//txsighashes houses the partial set of sighashes introduced within bip0143
//this partial set of sighashes may be re-used within each input across a
//transaction when validating all inputs .as a result validation complexity
//for sighashall can be redued by a polynomial factor
type TxSigHashes struct {
	HashPrevOuts chainhash.Hash
	HashSequence chainhash.Hash
	HashOutputs  chainhash.Hash
}
