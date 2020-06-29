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
	"net"
	"sync"
	peerpkg "BtcoinProject/peer"
	"sync/atomic"
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

//issynccandidate returns whether or not the peer is a candidate to consider
//syncing from.
func (sm *SyncManager) isSyncCandidate(peer *peerpkg.Peer) bool {
	//typically a peer is not a candidate for sync if it is not a full node.
	//however regression test is special in that the regression tool is not
	//a full node and still needs to be consideered s sync candidate.
	if sm.chainParams == &chaincfg.ResgressionNetParams {
		//the peer is not a candidate if it is not coming from localhost
		//or the hostname can not be determined for some reason.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			return false
		}

		if host != "127.0.0.1" && host != "localhost" {
			return false
		}

	} else {
		//the peer is not a candidate fro sync if it is not a full node.
		//additionally ,if the segwit soft-fork package has activated. then
		//the peer must also be upgraded.
		segwitActive, err := sm.chain.IsDeploymentActive(chaincfg.DeploymentSegwit)
		if err != nil {
			log.Errorf("unable to query for message for segwit"+
				"soft-fork state:%v", err)
		}
		nodeServices := peer.Services()
		if nodeServices&wire.SFNodeNetwork != wire.SFNodeNetwork ||
			(segwitActive && !peer.IsWitnessEnabled()) {
			return false
		}

	}

	//candidate if all checks passed.
	return true

}

//handlenewpeermsg deals with new peers that have signnalled that may be considered 
//as a sync peer (they have already successfully negotifateed).it 
//also statrs syncing if needed.it is invoked from the sycnheader goroutine.
func (sm *SyncManager) handleNewPeerMsg(peer *peerpkg.Peer) {
	//ignore if in the process of shutting down.
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}

	log.Infof("new valid peer %s (%s)", peer, peer.UserAgent())

	//initialize the peer state.
	isSyncCandidate := sm.isSyncCandidate(peer)
	sm.peerStates[peer] = &peerSyncState{
		syncCandidate:   isSyncCandidate,
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}

	//start syncing by chossing the best candidate if needed.
	if isSyncCandidate && sm.syncPeer == nil {
		sm.startSync()
	}
}

//handlestallsample will switch to a new sync peer if the current one has
//stalled .this is detected when by comparing the last progerss timestamp
//with the current time.and disconnecting the peer if we stalled before reaching
//their hightest advertised block.
func (sm *SyncManager) handleStallSample {
	if atomic.LoadInt32(&sm.shutdown) != 0 {
		return
	}

	//if we do not have an active sync peer. exit early.
	if sm.syncPeer == nil {
		return
	}

	//if the stall timeout has not elapsed .exit early.
	if time.Since(sm.lastProgressTime) <= maxStallDuration {
		return
	}

	//check to see that the peer sync state exists.
	state, exists := sm.peerStates[sm.syncPeer]
	if !exists {
		return
	}

	sm.clearRequestedState(state)

	disconnectSyncPeer := sm.shouldDCStalledSyncPeer()
	sm.updateSyncPeer(disconnectSyncPeer)

}

//shoulddscstallsyncpeer determines whether or not we should disconnect a
//stalled sync peer.if the peer has stalled and its reported height is gerater
//than our oun best height we will disconnect it.otherwise .wei will keep the
//peer connected in case we are already at tip
func (sm *SyncManager) shouldDSctalledSyncpeer() bool {
	lastBlock := sm.syncPeer.LastBlock()
	startHeight := sm.syncPeer.StartingHeight()

	var peerHeight int32
	if lastBlock > startHeight {
		peerHeight = lastBlock
	} else {
		peerHeight = startHeight
	}

	//if we have stalled out yet the sync peer reports having more blocks for
	//us we will disconnect them. this allows us at tip to not disconnect
	//peers when we are equal or they temporarily lay behind us
	best := sm.chain.BestSnapshot()
	return peerHeight > best.Height

}

//handledongepeermsg deals with peers that have singalled they are dong
//it removes the peer as a candidate for syncing and in the case where it
//was the current sync peer.attempts to select a new best peer to sync from.
//it is invoked form the synchandler goroutine.
func (sm *SyncManager) handleDonePeerMsg(peer *peerpkg.Peer) {

	state, exists := sm.peerStates[peer]
	if !exists {
		log.Warnf("received done peer message for unkonwn peer %s ", peer)
		return
	}

	//remvoe the peer from the list of candidate peers.
	delete(sm.peerStates, peer)

	log.Infof("lost peer %s", peer)

	sm.clearRequestedState(state)

	if peer == sm.syncPeer {
		//update the sync peer.the server has already disconnected the
		//peer before singaling to the sync manager.
		sm.updateSyncPeer(false)
	}

}

//clearrequeststate wipes all expected transactions and blocks from the sync
//manager,s requested maps that were requested under a peer sync state. this
//allows them to be requested by a subsequent sync peer.
func (sm *SyncManager) clearRequestedState(state *peerSyncState) {
	//remove requested transactions from the global map so that they will
	//be fetched from elsewhere next time we get an inv.
	for txHash := range state.requestedTxns {
		delete(sm.requestedTxns, txHash)

	}

	//remove requested blocks from the global map so that they will be
	//fetched from elsewhere next time we get an inv.
	//and request them now to speed things up a little.
	for blockHash := range state.requestedBlocks {
		delete(sm.requestedBlocks, blockHash)
	}

}

//updatesyncpeer choose a new sync peer to replace the curent one.if
//dcsyncpeer is true.this method will also disconnect the curent sync peer.
//if we are in header first mode.any header state related to prefetching is
//aslo reset in preparation for the next sync peer.
func (sm *SyncManager) updateSyncPeer(dcSyncPeer bool) {
	log.Debugf("updating sync peer.no progres for :%v", time.Since(sm.lastProgressTime))

	//first ,disconnect the current sync peer if requested .
	if dcSyncPeer {
		sm.syncPeer.Disconnect()
	}

	//reset any header state before we choose our next active sync peer.
	if sm.headersFirstMode {
		best := sm.chain.BestSnapshot()
		sm.resetHeaderState(&best.Hash, best.Height)

	}

	sm.syncPeer = nil
	sm.startSync()

}

//handletxmsg handles transaction message from all peers.
func (sm *SyncManager) handleTxMsg(tmsg *txMsg) {
	peer := tmsg.peer
	state, exists := sm.peerStates[peer]
	if !exists {
		log.Warnf("received tx message from unkonown peer %s", peer)
		return
	}

	// NOTE:  BitcoinJ, and possibly other wallets, don't follow the spec of
	// sending an inventory message and allowing the remote peer to decide
	// whether or not they want to request the transaction via a getdata
	// message.  Unfortunately, the reference implementation permits
	// unrequested data, so it has allowed wallets that don't follow the
	// spec to proliferate.  While this is not ideal, there is no check here
	// to disconnect peers for sending unsolicited transactions to provide
	// interoperability.
	txHash := tmsg.tx.Hash()

	//ignore transactions that we have already rejected . do not
	//send a reject message here because if the transaction was
	//rejected .the transaction was unsolicited.

	if _, exists = sm.requestedTxns[*txHash]; exists {
		log.Debugf("ignoring unsolicited previously rejeceted "+
			"transaction %v from %s", txHash, peer)
		return
	}

	//process the transaction to inculude validation ,insertion in the
	//memory pool. orphan handling ,etc.
	accepedTxs, err := sm.txMemPool.ProcessTransaction(tmsg.tx,
		true, true, mempool.Tag(peer.ID()))

	//remove transaction from request maps ,either the mempool/chain
	//already knows about it and as such we should not have any more
	//instances of trying to fetch it .or we failed to insert and thus
	//we will retry next time we get an inv.
	delete(state.requestedTxns, *txHash)
	delete(sm.requestedTxns, *txHash)

	if err != nil {
		//do not request this trnasaction again until a new block has been processed.
		sm.requestedTxns[*txHash] = struct{}{}
		sm.limitMap(sm.requestedTxns, maxRejectedTxns)

		//when the error is a rule error, it means the transaction was
		//simply rejected as opposed to something actually going wrong.
		//so log it as such.otherwise.something really did go wrong .
		//so log it as an actual error.
		if _, ok := err.(mempool.RuleError); ok {
			log.Debugf("rejected transaction %v from %s:%v", txHash, peer, err)

		} else {
			log.Errorf("failed to precess transaction %v :%v", txHash, err)

		}

		//convert the error into an appropriate reject message and send it
		code, reason := mempool.ErrToRejectErr(err)
		peer.PushRejectMsg(wire.CmdTx, code, reason, txHash, false)
		return

	}

	sm.PeerNotifier.AnnounceNewTransacitions(accepedTxs)

}

//curent returns true if we believe we are synced with our peers .false if
//still have blocks to check.

func (sm *SyncManager) current() bool {
	if !sm.chain.IsCurrent() {
		return false
	}

	//if blockchain thinks we are current and we have no syncpeer it
	//is probably right.
	if sm.syncPeer == nil {
		return true
	}

	//no matter what chain thinks,if we are below the block we are syncing
	//to we are not curent
	if sm.chain.BestSnapshot().Height < sm.syncPeer.LastBlock() {
		return false
	}
	return true
}

//handleblockmsg handles block messages from all peers.
func (sm *SyncManager) handleBlockMsg(bmsg *blockMsg) {

	peer := bmsg.peer
	state, exists := sm.peerStates[peer]
	if !exists {
		log.Warnf("receiver block message from unkonow peer %s", peer)
		return
	}

	//if we did not ask for this block then the peer is misbehaving.
	blockHash := bmsg.block.Hash()
	if _, exists = state.requestedBlocks[*blockHash]; !exists {
		//the regresssin text intentionally sends some blocks twice
		//to test duplicate blcok inseraction fails. do not disconnect
		//the peer or ignore the block when we are in regression test
		//mode in this case so the chain code is actually fed the duplicate
		//blocks.
		if sm.chainParams != &chaincfg.ResgressionNetParams {
			log.Warnf("got unrequested block %v from %s --"+
				"disconnecting", blockHash, peer.Addr())
			peer.Disconnect()
			return
		}

	}

	//when in header-first mode.if the block mateches the hash of the first
	//header in the list of headers that are being fetched.it is eligible for
	//less validation since the headers have already been verrified to link
	//together and are valid up to the next checkpoint since it is needed to
	//verrify the next round of headers links properly
	isCheckpointBlock := false
	behaviorFlags := blockchain.BFNone
	if sm.headersFirstMode {
		firstNodeEl := sm.headerList.Front()
		if firstNodeEl != nil {
			firstNode := firstNodeEl.Value.(*headerNode)
			if blockHash.IsEqual(firstNode.hash) {
				behaviorFlags |= blockchain.BFFastAdd
				if firstNode.hash.IsEqual(sm.nextCheckpoint.Hash) {
					isCheckpointBlock = true
				} else {
					sm.headerList.Remove(firstNodeEl)
				}
			}
		}
	}

	//remove block from request maps,either chain will know about it and
	//so we should not have more instances of trying to fetch it .or we
	//will fail the insert and thus we will retry next time we get an inv.
	delete(state.requestedBlocks, *blockHash)
	delete(sm.requestedBlocks, *blockHash)

	//process the block to include validation.best chain selection.orphan
	//handling.etc.
	_, isOrphan, err := sm.chain.ProcessBlock(bmsg.block, behaviorFlags)
	if err != nil {
		//when the error is a rule error,it means the block was simply
		//rejected as opposed to something actully going wrong.so log
		//it as such.otherwise ,something really did go wrong,so log
		//it as an actual error.
		if _, ok := err.(blockchain.RuleError); ok {
			log.Infof("rejected block %v from %s :%v", blockHash,
				peer, err)
		} else {
			log.Errorf("failed to process block %v:%v", blockHash, err)
		}
		if dbErr, ok := err.(database.Error); ok && dbErr.ErrorCode ==
			database.ErrCorruption {
			panic(dbErr)
		}

		//convert the error into an appropaiate reject message and send it
		code, reason := mempool.ErrToRejectErr(err)
		peer.PushRejectMsg(wire.CmdBlock, code, reason, blockHash, false)
		return

	}

	//meta-data about the new block this peer is reporting .we use this below
	//to update this peer latest block height and the heights of other peers
	//based on their last announced block hash.this allows us to dymamiccly update
	//the block heights of peers.avoiding stale heights when looking for a new sync
	//peer upon.acctptance of a block or recognition of an orphan peers who is
	//inv may have been ignored if we are actively syncing while the chain is yet
	//current or who may have lost the lock announcement race.
	var heightUpdate int32
	var blkHashUpdate *chainhash.Hash

	//request the parents for the orphan block from the peer that sent it.
	if isOrphan {
		//we have just received an orphan block from a peer.in order tu opdate
		//the height of the peer .we try to extract the block height from the
		//scriptSig of the coinbase transaction .extraction is only attempted if
		//if the block version is high enough.
		header := &bmsg.block.MsgBlock().Header
		if blockchain.ShouldHaveSerializedBlockHeight(header) {
			coinbaseTx := bmsg.block.Transactions()[0]
			cbHeight, err := blockchain.ExtractCoinbaseHeight(coinbaseTx)
			if err != nil {
				log.Warnf("unable to extract height from "+
					"coinbase tx:%v", err)
			} else {
				log.Debugf("extracted heighy of %v from "+
					"orphan block", cbHeight)
				heightUpdate = cbHeight
				blkHashUpdate = blockHash

			}
		}

		orphanRoot := sm.chain.GetGetOrphanRoot(blockHash)
		locator, err := sm.chain.LatestBlockLocator()
		if err != nil {
			log.Warnf("Failed to get block locator for the "+
				"latest block: %v", err)
		} else {
			peer.PushGetBlocksMsg(locator, orphanRoot)
		}

	} else {

		if peer == sm.syncPeer {
			sm.lastProgressTime = time.Now()
		}

		//when the block is not an orphan ,log information about it and
		//update the chain state.
		sm.progressLogger.LogBlockHeight(bmsg.block)

		//update this peer is latest block height .for future
		//potential sync node candidacy
		best := sm.chain.BestSnapshot()
		heightUpdate = best.Height
		blkHashUpdate = &best.Hash

		//clear the rejected transaction
		sm.rejectedTxns = make(map[chainhash.Hash]struct{})

	}

	//update the block height for this peer.but only send a message to
	//the server for upadating peer hegihts if this is an orphan or our
	//chain is "current	".this avoids sending a spammy amout of messages
	//if we are syncing the chain from scratch.
	if blkHashUpdate != nil && heightUpdate != 0 {
		peer.UpdateLastBlockHeight(heightUpdate)
		if isOrphan || sm.current() {
			go sm.PeerNotifier.UpdataPeerHeights(blkHashUpdate, heightUpdate, peer)
		}
	}

	//nothing more to do if we are not in headers-first  mode.
	if !sm.headersFirstMode {
		return
	}

	//this is header-first mode .so if the block is not a checkpoint
	//request more blcoks using the header list when the request queue
	//is getting short.
	if !isCheckpointBlock {
		if sm.startHeader != nil &&
			len(state.requestedBlocks) < minInFlightBlocks {
			sm.fetchHeaderBlocks()
		}
		return
	}

	//this is header-first mode and the block is a checkpoint.when there is
	//a next checkpoint.get the next round of headers by asking for headers
	//starting from the block after this one up to the next checkpoint.
	prevHeight := sm.nextCheckpoint.Height
	prevHash := sm.nextCheckpoint.Hash
	sm.nextCheckpoint = sm.findNextHeaderCheckpoint(prevHeight)
	if sm.nextCheckpoint != nil {
		locator := blockchain.BlockLocator([]*chainhash.Hash{prevHash})
		err := peer.PushGetHeadersMsg(locator, sm.nextCheckpoint.Hash)
		if err != nil {
			log.Warnf("failed to send getheaders message to "+
				"peer %s :%v", peer.Addr(), err)
			return
		}

		log.Infof("downloading headers for blocks %d to %d from"+
			"peer %s", prevHeight+1, sm.nextCheckpoint.Height, sm.syncPeer.Addr())

		return

	}

	//this is header-first mode.the block is a checkpoint ,and there are
	//no more checkpoint.so switch to normal mode by requesting blcoks
	//from the block after this one up to end of chain (zero hash)
	sm.headersFirstMode = false
	sm.headerList.Init()
	log.Infof("reached the final checkpoint -- switch to normal mode ")
	locator := blockchain.BlockLocator([]*chainhash.Hash{blockHash})
	err = peer.PushGetBlocksMsg(locator, &zeroHash)
	if err != nil {
		log.Warnf("failed to send getblocks message to peer %s:%v",
			peer.Addr(), err)
		return
	}

}

//fetchheaderblocks creates and sends a request to the syncpeer for the next
//list of blocks to be downloaded based on the current list of headers.
func (sm *SyncManager) fetchHeaderBlocks() {
	//nothing to do if there is no start header.
	if sm.startHeader == nil {
		log.Warnf("fetchheaderblcoks called with no start header")
		return
	}

	//build up a getdata requested for the list of blocks the headers
	//describe .the size hint will be limited to wire.maxinvpermsg by
	//the function.so no need to double check it here.

	gdmsg := wire.NewMsgGetDataSizeHint(uint(sm.headerList.Len()))
	numRequested := 0
	for e := sm.startHeader; e != nil; e = e.Next() {
		node, ok := e.Value.(*headerNode)
		if !ok {
			log.Warnf("header list node type is not a headernode")
			continue
		}

		iv := wire.NewInvVect(wire.InvTypeBlock, node.hash)
		haveInv, err := sm.haveInventory(iv)
		if err != nil {
			log.Warnf("unexpected failure when checking for "+
				"existing inventory during header block"+
				"fetch:%v", err)
		}
		if !haveInv {
			syncPeerState := sm.peerStates[sm.syncPeer]

			sm.requestedBlocks[*node.hash] = struct{}{}
			syncPeerState.requestedBlocks[*node.hash] = struct{}{}

			//if we are fetching from a witness enabled peer post-fork.
			//then ensure that we recive all the witness data in the blocks.
			if sm.syncPeer.IsWitnessEnabled() {
				iv.Type = wire.InvTypeWitnessBlock
			}

			gdmsg.AddInvVect(iv)

			numRequested++

		}

		sm.startHeader = e.Next()
		if numRequested >= wire.MaxInvPerMsg {
			break
		}

	}
	if len(gdmsg.InvList) > 0 {
		sm.syncPeer.QueueMessage(gdmsg, nil)
	}

}

//handleheadermsg handles block header message from all peers, headers
//are requested when performing a headers-first sync.
func (sm *SyncManager) handleHeadersMsg(hmsg *headersMsg) {
	peer := hmsg.peer
	_, exists := sm.peerStates[peer]
	if !exists {
		log.Warnf("received headers message from unknown peer %s", peer)
		return
	}

	//the remote peer is misbehaving if we did not request headers .
	msg := hmsg.headers
	numHeaders := len(msg.Headers)
	if !sm.headersFirstMode {
		log.Warnf("get %d unrequested headers from %s --"+
			"disconnecting ", numHeaders, peer.Addr())
		peer.Disconnect()
		return
	}

	//nothing to do for an empty headers message.
	if numHeaders == 0 {
		return
	}

	//process all of the received headers ensuring each one connects to the
	//previous and that checkpoints match
	receivedCheckpoint := false
	var finalHash *chainhash.Hash
	for _, blockHeader := range msg.Headers {
		blockHash := blockHeader.BlockHash()
		finalHash = &blockHash

		//ensure there is a previous headers to compare against.
		prevNodeEl := sm.headerList.Back()
		if prevNodeEl == nil {

			log.Warnf("header list does not contain a previous " +
				"element as expected -- disconnecting peer")
			peer.Disconnect()
			return
		}

		//ensure the header properly connects to the previous one and
		//add it to the list of headers
		node := headerNode{hash: &blockHash}
		prevNode := prevNodeEl.Value.(*headerNode)
		if prevNode.hash.IsEqual(&blockHeader.PrevBlock) {
			node.height = prevNode.height + 1
			e := sm.headerList.PushBack(&node)
			if sm.startHeader == nil {
				sm.startHeader = e
			}
		} else {

			log.Warnf("received block headr that does not"+
				"properly connect to the chain from peer %s"+
				"--disconnecting ", peer.Addr())
			peer.Disconnect()
			return

		}

		//verify the header at the next checkpoint height matches.
		if node.height == sm.nextCheckpoint.Height {
			if node.hash.IsEqual(sm.nextCheckpoint.Hash) {
				receivedCheckpoint = true
				log.Infof("verified downloading block"+
					"header against checkpoint at height "+
					"%d/hash %s", node.height, node.hash)
			} else {

				log.Warnf("block header at height %d/hash"+
					"%s from peer %s does not match "+
					"expected chenckpoint hash of %s --"+
					"disconnectging", node.height, node.hash, peer.Addr(), sm.nextCheckpoint())
				peer.Disconnect()
				return

			}
			break
		}

	}

	//when this header is a checkpoint ,switch to fetcting the blocks for
	//all of the headers since the last checkpoint.
	if receivedCheckpoint {
		//since the first entry of the list is always the final block
		//that is already in the database and is only used to ensure
		//the next header links properly.it must be removed before fetching
		//the blocks.
		sm.headerList.Remove(sm.headerList.Front())
		log.Infof("received %v block headers :fecthing blocks", sm.headerList.Len())

		sm.progressLogger.SetLastLogTime(time.Now())
		sm.fetchHeaderBlocks()
		return
	}

	//this header is not a checkpoint .so request the next batch of headers
	//starting from the latest konwn header and ending with the next checkpoint
	locator := blockchain.BlockLocator([]*chainhash.Hash{finalHash})
	err := peer.PushGetHeadersMsg(locator, sm.nextCheckpoint.Hash)
	if err != nil {
		log.Warnf("failed to send getheaders message to "+
			"peer %s:%v", peer.Addr(), err)
		return
	}

}

//haveinventory returns whether or not the inventory represented by the
//passed inventory vector is known .this include checking all of the various
//places invnetroy can be when it is in different states such as blcoks that
//part of then main chain. on a side chain. in the orphan pool.and transaction that
//are in the memory pool.(either the main pool or orphan pool)
func (sm *SyncManager) haveInventory(invVect *wire.InvVect) (bool, error) {

	switch invVect.Type {
	case wire.InvTypeWitnessBlock:
		fallthrough
	case wire.InvTypeBlock:
		//ask chain if the block if known to it in any form(mian chain)
		//,side chain. or orphan.
		return sm.chain.HaveBlock(&invVect.Hash)

	case wire.InvTypeWitnessTx:
		fallthrough
	case wire.InvTypeTx:
		//ask the tranasction memeory pool if the transaction is known to it
		//in any form (main pool or orphan).
		if sm.txMemPool.HaveTransaction(&invVect.Hash) {
			return true, nil
		}

		//check if the transaction exists from the point of view of the
		//end of the main chain.note that this is only a best effort
		//since it is expensive to check existence of every output and
		//the only purpose of this check is to avoid downloading already
		//known transactions .only the first two outputs are checked
		//because the vast majority of transaction consists of two
		//outputs where one is some form of pay to somebody else and
		//the other is a change output.

		prevOut := wire.OutPoint{Hash: invVect.Hash}
		for i := uint32(0); i < 2; i++ {
			prevOut.Index = i
			entry, err := sm.chain.FetchUtxoEntry(prevOut)
			if err != nil {
				return false, err
			}
			if entry != nil && entry.isSpent() {
				return true, nil
			}
		}

		return false, nil
	}

	//the requested inventory is an unsupported type,so just claim
	//it is known to avoid requesting it.
	return true, nil

}

//handleinvmsg handles inv message from all peers.
//we examine the invetory advertised by the remote peer and act accordingly.
func (sm *SyncManager) handleInvMsg(imsg *invMsg) {
	peer := imsg.peer
	state, exists := sm.peerStates[peer]
	if !exists {
		log.Warnf("received inv message from unkonwn peer %s", peer)
		return
	}

	//attempt to find the final block in the inventory list.there may not
	//not be one.
	lastBlock := -1
	invVects := imsg.inv.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			lastBlock = i
			break
		}
	}

	//if this contains a block announcement. and this isn,t coming from
	//our current sync peer or we are current.then update the last annoucement.
	//block for this peer.we will use this information later to update the
	//height of peers based on blocks we have accepted that they previously
	//annoucemed.
	if lastBlock != -1 && (peer != sm.syncPeer || sm.current()) {
		peer.UpdateLastAnnouncedBlock(&invVects[lastBlock].Hash)
	}

	//ingnor invs from peers that are not the sync if we are not current
	//helps prevent fetching a mass of orphans.
	if peer != sm.syncPeer && !sm.current() {
		return
	}

	//if our chain is current and a peer annouces a block we already
	//know of .then update their current block height .
	if lastBlock != -1 && sm.current() {
		blkHeight, err := sm.chain.BlockHeightByHash(&invVects[lastBlock].Hash)
		if err == nil {
			peer.UpdateLastBlockHeight(blkHeight)
		}
	}

	//request the advertised inventory if we do not alreay have it .also
	//request parent blocks of orphans if we receive one we already have.
	//finally.attempt to detect potential stalls due to long side chains
	//we already have and request more blocks to prevent them.
	for i, iv := range invVects {
		//ignore unsupprted inventory types.
		switch iv.Type {
		case wire.InvTypeBlock:
		case wire.InvTypeTx:
		case wire.InvTypeWitnessBlock:
		case wire.InvTypeWitnessTx:
		default:
			continue

		}

		//add the inventory to the cache of known invntory for the peer.
		peer.AddKnownInventory(iv)

		//innore inventory when we are in header-first mode.
		if sm.headersFirstMode {
			continue
		}

		//request the inventroy if we do not already have it
		haveInv, err := sm.haveInventory(iv)
		if err != nil {
			log.Warnf("unexpected failure when checking for "+
				"existing inventroy during inv messag "+
				"procesing :%v", err)
			continue
		}

		if !haveInv {
			if iv.Type == wire.InvTypeTx {
				//skip the transaction if it has already been rejected
				if _, exists := sm.requestedTxns[iv.Hash]; exists {
					continue
				}

			}

			//ignore invs block invs from non-witness enabled
			//peer as after segwit activation we only want to
			//download from peers that can provide us full witness
			//data for blocks.
			if !peer.IsWitnessEnable() && iv.Type == wire.InvTypeBlock {
				continue
			}

			//add it to the request queue
			state.requestQueue = append(state.requestQueue, iv)
			continue

		}

		if iv.Type == wire.InvTypeBlock {

			//the block is an orphan block that we already have .
			//when the existing orphan was precessed ,t requested
			// the missing parent blocks.  When this scenario
			// happens, it means there were more blocks missing
			// than are allowed into a single inventory message.  As
			// a result, once this peer requested the final
			// advertised block, the remote peer noticed and is now
			// resending the orphan block as an available block
			// to signal there are more missing blocks that need to
			// be requested.
			if sm.chain.IsKnownOrphan(&iv.Hash) {
				//request blocks starting at the latest known
				//up to the root of the orphan that just came
				//in.
				orphanRoot := sm.chain.GetOrphanRoot(&iv.Hash)
				locator, err := sm.chain.LatestBlockLocator()
				if err != nil {
					log.Errorf("PEER: Failed to get block "+
						"locator for the latest block: "+
						"%v", err)
					continue
				}
				peer.PushGetBlocksMsg(locator, orphanRoot)
				continue
			}

			// We already have the final block advertised by this
			// inventory message, so force a request for more.  This
			// should only happen if we're on a really long side
			// chain.
			if i == lastBlock {
				// Request blocks after this one up to the
				// final one the remote peer knows about (zero
				// stop hash).
				locator := sm.chain.BlockLocatorFromHash(&iv.Hash)
				peer.PushGetBlocksMsg(locator, &zeroHash)

			}

		}

	}

	//request as much as possible at once ,anything that wont not fit ninto
	//the request will be requested on the next inv message
	numRequested := 0
	gdmsg := wire.NewMsgGetData()
	requestQueue := state.requestQueue
	for len(requestQueue) != 0 {
		iv := requestQueue[0]
		requestQueue[0] = nil
		requestQueue = requestQueue[1:]

		switch iv.Type {
		case wire.InvTypeWitnessBlock:
			fallthrough
		case wire.InvTypeBlock:
			//request the block if there is not already a pending request
			if _, exists := sm.requestedBlocks[iv.Hash]; !exists {
				sm.requestedBlocks[iv.Hash] = struct{}{}
				sm.limitMap(sm.requestedBlocks, maxRequestedBlocks)
				state.requestedBlocks[iv.Hash] = struct{}{}

				if peer.IsWitnessEnabled() {
					iv.Type = wire.InvTypeWitnessBlock
				}

				gdmsg.AddInvVect(iv)
				numRequested++
			}

		case wire.InvTypeWitnessTx:
			fallthrough
		case wire.InvTypeTx:
			//request the transaction if there is not already a
			//pending request

			if _, exists := sm.requestedTxns[iv.Hash]; !exists {
				sm.requestedTxns[iv.Hash] = struct{}{}
				sm.limitMap(sm.requestedTxns, maxRequestedTxns)
				state.requestedTxns[iv.Hash] = struct{}{}

				//if the peer is capable ,request the txn including
				//all witness data.

				if peer.IsWitnessEnabled() {
					iv.Type = wire.InvTypeWitnessTx
				}

				gdmsg.AddInvVect(iv)
				numRequested++

			}

		}

		if numRequested >= wire.MaxInvPerMsg {
			break
		}

		state.requestedQueue = requestQueue
		if len(gdmsg.InvList) > 0 {
			peer.QueueMessage(gdmsg, nil)
		}

	}

}
