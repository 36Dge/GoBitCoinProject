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
func (b *BlockChain)maybeAcceptBlock(block *btcutil.Block,flags BehaviorFlags) (bool,error) {
	//the height of this block is one more than the referenced previous block
	prevHash := &block.MsgBlock().Header.PrevBlock
	prevNode := b.index.LookupNode(prevHash)
	if prevNode == nil{
		str := fmt.Sprintf("previus block %s is unkonown ",prevHash)
		return false,ruleError(ErrPreviousBlockUnknown,str)
	} else if b.index.NodeStatus(prevNode).KnownInvalid(){
		str := fmt.Sprintf("previous block %s is known to be inbalid",prevHash)
		return false,ruleError(ErrInvalidAncestorBlock,str)
	}

	blockHeight := prevNode.height + 1
	block.SetHeight(blockHeight)

	//the block must pass all of the validation rules which depend on the
	//position of the block within the block chain.
	err := b.checkBlockContext(block, prevNode, flags)
	if err != nil{
		return false,err
	}


























}