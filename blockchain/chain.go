package blockchain

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"sync"
	"time"
)

//BestState包含有关当前最佳块的信息和其他信息
//从当前最佳块。
//BestSnapshot方法可用于获取此信息的访问权限
//以并发安全的方式，数据不会从下更改。
//按照函数名的含义，当链状态发生更改时调用方。
//但是，返回的快照必须被视为不可变的，因为它是
//由所有呼叫者共享。

type BestState struct {
	Hash        chainhash.Hash // 块的哈希值
	Height      int32          // 块的高度
	Bits        uint32         // 块的难度
	BlockSize   uint64         //块的大小
	BlockWeight uint64         // 块的重量
	NumTxns     uint64         // 块中的txn的数组
	TotalTxns   uint64         //链中Txn的总数
	MedianTime  time.Time      // 根据calcpastmediantime确定的中间时间
}

type BlockLocator []*chainhash.Hash

//区块链提供使用比特币区块链的功能
//它包括拒绝重复块，确保块等功能
//遵循所有规则，孤立处理，检查点处理和最佳链
//选择与重组
type BlockChain struct {
	//以下字段是在创建实例时设置的，之后不能再更改
	//因此无需使用单独互斥
	checkpoints         []chaincfg.Checkpoint
	checkpointsByHeight map[int32]*chaincfg.Checkpoint
	db                  database.DB
	chainParams         *chaincfg.Params
	timeSource          MedianTimeSource
	sigCache            *txscript.SigCache
	indexManager        IndexManager
	hashCache           *txscript.HashCache
	//以下字段是根据提供的链计算的参数，他们也在创建实例
	//时候设置,并且以后不能再更改，因此无需使用单独互斥体
	minRetargetTimespan int64
	maxRetargetTimespan int64
	blockPerRetarget    int32

	//chainlock保护大多数此结构体中低于此点的字段
	chainLock sync.RWMutex
	//他们自己的锁，但是他们也经常受到链条的保护
	//锁定以帮助在处理块时防止逻辑争用。
	//索引将整个块索引存储在内存中。块索引是树形结构
	//bestchain通过使用块索引中的高效链视图
	index     *blockIndex
	bestChain *chainView

	//这些字段与处理孤立块相比，他们是由联锁和孤立锁的组合保护
	orphanLock   sync.RWMutex
	orphans      map[chainhash.Hash]*orphanBlock
	prevOrphans  map[chainhash.Hash][]*orphanBlock
	oldestOrphan *orphanBlock

	// These fields are related to checkpoint handling.  They are protected
	// by the chain lock.
	nextCheckpoint *chaincfg.Checkpoint
	checkpointNode *blockNode


	//状态被用作缓存信息的一种相当有效的方法。
	//关于在以下情况下返回给调用方的当前最佳链状态：
	//请求。它的工作原理是MVCC，因此任何时候
	//新块成为最佳块，状态指针替换为
	//新结构和旧状态保持不变。这样，
	//多个调用方可以指向不同的最佳链状态。
	//对于大多数呼叫者来说，这是可以接受的，因为状态
	//在特定时间点查询。此外，一些字段存储在数据库中，因此
	//链状态可以在加载时快速重建。

	stateLock     sync.RWMutex
	stateSnapshot *BestState

	//以下缓存用于有效地跟踪
	//每个规则的当前部署阈值状态将更改部署。
	//此信息存储在数据库中，因此可以快速带载重建。
	//警告缓存缓存块的当前部署阈值状态
	//在每个**可能的**部署中。这是用来
	//在投票表决新的未识别规则更改时检测和/或
	//已被激活，如在旧版本的软件正在使用中
	//DeploymentCaches缓存的当前部署阈值状态
	//每个活动定义的部署中的块。

	warningCaches    []thresholdStateCache
	deploymentCaches []thresholdStateCache

	//以下字段用于确定某些警告是否具有已经显示。
	//未知规则是指由于未知规则激活。
	//UnknownInversionsWarned是指由未知版本引起的警告。
	//正在开采。
	unknownRulesWarned    bool
	unknownVersionsWarned bool

	//“通知”字段存储要在其上执行的回调切片
	//某些区块链事件。
	notificationsLock sync.RWMutex
	notifications     []NotificationCallback
}

//孤立块表示我们还没有父块的块。它是一个普通块加上
//一个过期时间以防止缓存孤立块永远

type orphanBlock struct {
	block      *btcutil.Block
	expiration time.Time
}



// GetOrphanRoot returns the head of the chain for the provided hash from the
// map of orphan blocks.
//
// This function is safe for concurrent access.
func (b *BlockChain) GetOrphanRoot(hash *chainhash.Hash) *chainhash.Hash {
	// Protect concurrent access.  Using a read lock only so multiple
	// readers can query without blocking each other.
	b.orphanLock.RLock()
	defer b.orphanLock.RUnlock()

	// Keep looping while the parent of each orphaned block is
	// known and is an orphan itself.
	orphanRoot := hash
	prevHash := hash
	for {
		orphan, exists := b.orphans[*prevHash]
		if !exists {
			break
		}
		orphanRoot = prevHash
		prevHash = &orphan.block.MsgBlock().Header.PrevBlock
	}

	return orphanRoot
}



// IsKnownOrphan returns whether the passed hash is currently a known orphan.
// Keep in mind that only a limited number of orphans are held onto for a
// limited amount of time, so this function must not be used as an absolute
// way to test if a block is an orphan block.  A full block (as opposed to just
// its hash) must be passed to ProcessBlock for that purpose.  However, calling
// ProcessBlock with an orphan that already exists results in an error, so this
// function provides a mechanism for a caller to intelligently detect *recent*
// duplicate orphans and react accordingly.
//
// This function is safe for concurrent access.
func (b *BlockChain) IsKnownOrphan(hash *chainhash.Hash) bool {
	// Protect concurrent access.  Using a read lock only so multiple
	// readers can query without blocking each other.
	b.orphanLock.RLock()
	_, exists := b.orphans[*hash]
	b.orphanLock.RUnlock()

	return exists
}



// BlockLocatorFromHash returns a block locator for the passed block hash.
// See BlockLocator for details on the algorithm used to create a block locator.
//
// In addition to the general algorithm referenced above, this function will
// return the block locator for the latest known tip of the main (best) chain if
// the passed hash is not currently known.
//
// This function is safe for concurrent access.
func (b *BlockChain) BlockLocatorFromHash(hash *chainhash.Hash) BlockLocator {
	b.chainLock.RLock()
	node := b.index.LookupNode(hash)
	locator := b.bestChain.blockLocator(node)
	b.chainLock.RUnlock()
	return locator
}



// LatestBlockLocator returns a block locator for the latest known tip of the
// main (best) chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) LatestBlockLocator() (BlockLocator, error) {
	b.chainLock.RLock()
	locator := b.bestChain.BlockLocator(nil)
	b.chainLock.RUnlock()
	return locator, nil
}


