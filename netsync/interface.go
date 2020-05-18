package netsync

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/mempool"
	"github.com/btcsuite/btcutil"
)

//PeerNotifier公开方法中通知对等机状态
//更改当前服务器（在主包中）实现的事务、
//块等
type PeerNotifier interface {
	AnnounceNewTransacitions(newTxs []*mempool.TxDesc)
	UpdataPeerHeights(latestBlkhash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer)
	RelayInventory(invVect *wire.InvVect, data interface{})
	TransactionConfirmed(tx *btcutil.Tx)
}

//config是用于初始化新的SyncManager的配置结构
type Config struct {
	PeerNotifier       PeerNotifier
	Chain              *blokchain.BlockChain
	TxMemPool          *mempool.TxPool
	ChainParams        *chaincfg.Params
	DisableCheckpoints bool
	MaxPeers           int
	FeeEstimator       *mempool.FeeEstimator
}



