package chaincfg

import "BtcoinProject/chaincfg/chainhash"

type Checkpoint struct {
	Height int32
	Hash   *chainhash.Hash
}
