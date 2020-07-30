package mining

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
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

