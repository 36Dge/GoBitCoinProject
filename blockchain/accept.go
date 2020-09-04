package blockchain

import (
	"fmt"
	"github.com/btcsuite/btcutil"
)

//maybeacceiptblock potentially accept a block into the block chain
//and if accepted .returns whether or not it is on the chain .it performs
//several validation checks which depend on its positioin whthiin blcok
//chain before adding it.the block is expected to have arready gone
//through processblock before calling this function with it.
//the flags are also passed to checkblockcontext and concectbestchain .see
//their documentation for how the flags modify their behavior.
func (b *BlockChain) maybeAcceptBlock(block *btcutil.Block, flags BehaviorFlags) (bool, error) {
	//the height of this block is one more than the referenced previous block
	prevHash := &block.MsgBlock().Header.PrevBlock
	prevNode := b.index.LookupNode(prevHash)
	if prevNode == nil {
		str := fmt.Sprintf("previus block %s is unkonown ", prevHash)
		return false, ruleError(ErrPreviousBlockUnknown, str)
	} else if b.index.NodeStatus(prevNode).KnownInvalid() {
		str := fmt.Sprintf("previous block %s is known to be inbalid", prevHash)
		return false, ruleError(ErrInvalidAncestorBlock, str)
	}

	blockHeight := prevNode.height + 1
	block.SetHeight(blockHeight)

	//the block must pass all of the validation rules which depend on the
	//position of the block within the block chain.
	err := b.checkBlockContext(block, prevNode, flags)
	if err != nil {
		return false, err
	}

	//insert the block into the database if it is not already there .enen
	//though it is possible the block will ultimately fail to connect .it
	//has already passed all proof-of-work and validity tests which means
	//it would be prohibitively expensive for an attacker to fill up the
	//disk with a bunch of blocks that fail to conncet that is necessary
	//since it allows block download to be decoupled from the much more
	//expensive conncetion logic .it also has some other nice properties
	//such as making blocks that never become part of the main chain or
	//blocks that fail to conncet available for further analysis.
	err = b.db.Update(func(dbTx database.Tx) error {
		return dbStoreBlock(dbTx, block)
	})

	if err != nil {
		return false, err
	}

	//create a new block node for the block and add it to the node index ,even
	//if the block ultimately gets connceted to the main chain.it starts out
	//on a side chain.
	blockHeader := &block.MsgBlock().Header
	newNode := newBlockNode(blockHeader, prevNode)

	b.index.addNode(newNode)
	err = b.index.flushToDB()
	if err != nil {
		return false, err
	}

	//connect the passed block to the chain while respecting proper chain
	//selecting accourding to the chain with the most proof of work this
	//also handles validation of the transaction scripts.
	isMainChain, err := b.conncetBestChain(newNode, block, flags)
	if err != nil {
		return false, err
	}

	//notify the caller that the new block was accepted into the block
	//chain.the caller would typically want to react by relaying the inventory
	//to other peers
	b.chainLock.Unlock()
	b.sendNotification(NTBlockAccepted, block)
	b.chainLock.Lock()

	return isMainChain, nil
}

//over
