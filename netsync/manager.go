package netsync

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/mempool"
	"BtcoinProject/wire"
	"container/list"
	"github.com/btcsuite/btcutil"
	"math/rand"
	"sync"
	peerpkg "BtcoinProject/peer"
	"time"
)

const (
	//minflightBlock是应为在头的请求队列中，请求前的第一个模式
	minInFlightBlocks = 10
	//MaxRejectedTxns是被拒绝的事务的最大数目
	//哈希值存在内存中
	maxRejectedTxns = 1000
	//MaxRequestedBlocks是请求的最大块数
	//哈希值存储在内存中。
	maxRequestedBlocks = wire.MaxInvPerMsg

	//MaxRequestedTxns是请求的最大事务数
	//哈希值存储在内存中。
	maxRequestedTxns = wire.MaxInvPerMsg
)

// zero hash是零哈希值（全部为零）。它被定义在为一种便利
var zeroHash chainhash.Hash

//newpeermsg表示新连接到块处理程序的对等机
type newPeerMsg struct {
	peer *peerpkg.Peer
}

// blockmsg将比特币阻塞消息和它来自一起的对等放打包在一起
type blockMsg struct {
	block *btcutil.Block
	peer  *peerpkg.Peer
	relay chan struct{}
}

//invmsg将比特币inv消息和它来自一起的对等方打包
//所以块处理程序可以访问这些消息
type invMsg struct {
	inv  *wire.MsgInv
	peer *peerpkg.Peer
}

//headersmsg打包比特币头消息及其来自的对等方
//这样快处理程序就可以访问这些消息

type headersMsg struct {
	headers *wire.MsgHeaders
	peer    *peerpkg.Peer
}

//txmsg将比特币的tx消息和它来自同一个地方的同伴打包在一起
type txMsg struct {
	tx    *btcutil.Tx
	peer  *peerpkg.Peer
	reley chan struct{}
}

//getsyncpeermsg是要通过消息通道发送的消息类型
type getSyncPeerMsg struct {
	relay chan int32
}

//processBlockResponse 是发送到进程块消息
type processBlockResponse struct {
	isOrphan bool
	err      error
}

//processblockmsg是要通过消息通道发送的消息类型对于所请求的，将处理一个块。
//注意，此调用与blockmsg不同在上面的blockmsg中，用于来自对等方并且额外的处理，
//而此消息本质上只是一个并发安全的在内部块链实例上调用processBlock的方法。
type processBlockMsg struct {
	block *btcutil.Block
	flags blockchain.BehaviorFlags
	reply chan processBlockResponse
}

//isCurrentMsg is a message type to be sent
//across the message channel for
//请求同步管理器是否相信它与当前连接的对等机
type isCurrentMsg struct {
	reply chan bool
}

//pausemsg是要通过消息通过发送的消息类型
// 暂停同步管理器，这有效的为来电者提供了
//对管理器进行独占访问，直到取消暂停频道
type pauseMsg struct {
	unpause <-chan struct{}
}

type headerNode struct {
	height int32
	hash   *chainhash.Hash
}

//PeerSyncState存储Synmanager跟终的其他消息
type peerSyncState struct {
	syncCandidate   bool
	requestQueue    []*wire.InvVect
	requestedTxns   map[chainhash.Hash]struct{}
	requestedBlocks map[chainhash.Hash]struct{}
}

//syncManager用于与对等机端通信与快相关的消息。这个
//通过在goroutine中执行star()启动syscmanager。一旦
//开始一旦链同步，同步管理器处理传入的块和头通知并将新
//块的通知转发给对等方。
type SyncManager struct {
	PeerNotifier   PeerNotifier
	started        int32
	shutdown       int32
	chain          *blockchain.BlockChain
	txMemPool      *mempool.TxPool
	chainParams    *chaincfg.Params
	progressLogger *blockProgressLogger
	msgChan        chan interface{}
	wg             sync.WaitGroup
	quit           chan struct{}

	//这些字段只能从blockhandler线程访问
	rejectedTxns    map[chainhash.Hash]struct{}
	requestedTxns   map[chainhash.Hash]struct{}
	requestedBlocks map[chainhash.Hash]struct{}
	syncPeer        *peerpkg.Peer
	peerStates      map[*peerpkg.Peer]*peerSyncState

	//以下字段用于头一模式
	headersFirstMode bool
	headerList       *list.List
	startHeader      *list.Element
	nextCheckpoint   *chaincfg.Checkpoint

	//可选的费用估算器
	feeEstimator *mempool.FeeEstimator
}

//resetheaderstarte将头的第一模式状态设置为适合
//正在从新的对等机同步
func (sm *SyncManager) resetHeaderState(newestHash *chainhash.Hash, newestHeight int32) {
	sm.headersFirstMode = false
	sm.headerList.Init()
	sm.startHeader = nil
	//当有下一个检查点时，添加一个最新的已知条目阻止进入头池。这允许下一个下载的头文件
	//证明它与链条正确连接

	if sm.nextCheckpoint != nil {
		node := headerNode{height: newestHeight, hash: newestHash}
		sm.headerList.PushBack(&node)
	}

}

//findNextHeaderCheckpoint返回通过高度后的下一个检查点.
//当没有高度时，它返回零，因为高度已经迟与最终检查点或其他
//原因，如禁用检查点.
func (sm *SyncManager) findNextHeaderCheckpoint(height int32) *chaincfg.Checkpoint {
	checkpoints := sm.chain.Checkpoints()
	if len(checkpoints) == 0 {
		return nil
	}

	//如果高度在决赛之后，就没有下一个检查点了
	finalCheckpoint := &checkpoints[len(checkpoints)-1]
	if height >= finalCheckpoint.Height {
		return nil
	}

	//找到下一个检查点
	nextCheckpoint := finalCheckpoint
	for i := len(checkpoints) - 2; i >= 0; i-- {
		if height >= checkpoints[i].Height {
			break
		}
		nextCheckpoint = &checkpoints[i]

	}
	return nextCheckpoint
}

//startsync will choose the best peer among the available candidate
//peers to downlaod/sync the blockchain from.when sync is already running
//it simply retuns .it also examines the candicate for any which are no
//longer candicates and removers them sa needs.
func (sm *SyncManager) startSync() {
	//return now if we are already snncing
	if sm.syncPeer != nil {
		return
	}

	//once the segwit soft-fork package has activated.we only want to
	//sync from peers which are witness enabled to ensure that we fully
	//validate all blockchain data.
	segwitActive, err := sm.chain.IsDeploymentActive(chaincfg.DeploymentSegwit)
	if err != nil {
		log.Errorf("unable to query for segwit soft-fork sate :%v", err)
		return
	}

	best := sm.chain.BestSnapshot()
	var higherPeers, equalPeers []*peerpkg.Peer
	for peer, state := range sm.peerStates {
		if !state.syncCandidate {
			continue
		}

		if segwitActive && !peer.IsWitnessEnabled() {
			log.Debugf("peer %v not witness enabled,skipping ", peer)
			continue
		}

		//remove sync candidate peers that are no longer candidates due
		//to passing their latest known block. NOte:the < is international
		//does not have a later block when it is equal .it will likely have
		//one soon so it is a reasonable choice .it also allows the case
		//where both are at 0 such as during regerssion test.
		if peer.LastBlcok() < best.Heigth {
			state.syncCandidate = false
			continue
		}

		//if the peer is at the same height as us .we will add it a set
		//of backup peers in case we do not find one with a higher height
		//.if we are synced up with all of our peers all of them will be in
		//this set
		if peer.LastBlock() == best.Height {
			equalPeers = append(equalPeers, peer)
			continue
		}

		//this peer has a height greater than our oun .we will consider
		//it in the set of better peers from which we will randomly select.
		higherPeers = append(higherPeers, peer)

	}
	//pick randomly from the set of peers greater than our block hegit.
	//falling back to a random peer of the same height if none are greater
	//
	//oberved metric and / or sync in parallel.

	var bestPeer *peerpkg.Peer
	switch {
	case len(higherPeers) > 0:
		bestPeer = higherPeers[rand.Intn(len(higherPeers))]

	case len(equalPeers) > 0:
		bestPeer = equalPeers[rand.Intn(len(equalPeers))]

	}

	//start syncing from the best peer if one was selected.
	if bestPeer != nil {
		//clear the requestblocks if the sync peer changes.otherwise
		//we may ignore blocks we need that the last sync peer failed
		//to send.
		sm.requestedBlocks = make(map[chainhash.Hash]struct{})

		locator, err := sm.chain.LatestBlockLocator()
		if err != nil {
			log.Errorf("failed to get block locator for the "+
				"latest block :v", err)
			return
		}

		log.Infof("syncing to block height %d from peer %v",
			bestPeer.LastBlock(), bestPeer.Addr())

		// When the current height is less than a known checkpoint we
		// can use block headers to learn about which blocks comprise
		// the chain up to the checkpoint and perform less validation
		// for them.  This is possible since each header contains the
		// hash of the previous header and a merkle root.  Therefore if
		// we validate all of the received headers link together
		// properly and the checkpoint hashes match, we can be sure the
		// hashes for the blocks in between are accurate.  Further, once
		// the full blocks are downloaded, the merkle root is computed
		// and compared against the value in the header which proves the
		// full block hasn't been tampered with.
		//
		// Once we have passed the final checkpoint, or checkpoints are
		// disabled, use standard inv messages learn about the blocks
		// and fully validate them.  Finally, regression test mode does
		// not support the headers-first approach so do normal block
		// downloads when in regression test mode.
		if sm.nextCheckpoint != nil &&
			best.Height < sm.nextCheckpoint.Height &&
			sm.chainParams != &chaincfg.ResgressionNetParams {
			bestPeer.PushGetHeadersMsg(locator, sm.nextCheckpoint.Hash)
			sm.headersFirstMode = true
			log.Infof("downloading headers for blocks %d to "+
				"%d from peer %s ", best.Height+1, sm.nextCheckpoint.Height, best.Height+1,
				sm.nextCheckpoint.Height, bestPeer.Addr())
		} else {
			bestPeer.PushGetBlocksMsg(locator, &zeroHash)
		}
		sm.syncPeer = bestPeer
		// Reset the last progress time now that we have a non-nil
		// syncPeer to avoid instantly detecting it as stalled in the
		// event the progress time hasn't been updated recently.
		sm.lastProgressTime = time.Now()

	} else {
		log.Warnf("no sync peer candidates availalbe")
	}

}



