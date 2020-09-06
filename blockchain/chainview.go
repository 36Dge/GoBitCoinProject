package blockchain

import "sync"

//approxnodeperweek is an approximation of the number of new blcok there
//are in a week on average.
const approxNodeSPerWeek = 6 * 24 * 7

// log2FloorMasks defines the masks to use when quickly calculating
// floor(log2(x)) in a constant log2(32) = 5 steps, where x is a uint32, using
// shifts.  They are derived from (2^(2^x) - 1) * (2^(2^x)), for x in 4..0.
var log2FloorMasks = []uint32{0xffff0000, 0xff00, 0xf0, 0xc, 0x2}

//fastlog2floor calculates and returns floor(log2(x)) in a constant 5 steps.
func fastLog2Floor(n uint32) uint8 {
	rv := uint8(0)
	exponent := uint8(16)
	for i := 0; i < 5; i++ {
		if n&log2FloorMasks[i] != 0 {
			rv += exponent
			n >>= exponent
		}
		exponent >>= 1
	}
	return rv

}

//chainview provides a flat view of a specific branch of the block chain form
//its tip back to the genesis block and provides various convenience functions
//for comparing chains.

//for example ,assume a block chain with a side chain as depicted below:
//genesis  -> 1 -> 2 -> 3 -> 4  -> 5 ->  6  -> 7  -> 8
//                       \-> 4a -> 5a -> 6a

//the chain view for the branch ending in 6a consists of :
type chainView struct {
	mtx   sync.Mutex
	nodes []*blockNode
}

//newchainview returns a new chain view for the given tip block node.
//passing nil as the will result in a chain view that is not initialized
//the tip can be updated at any time via the settip function.
func newChainView(tip *blockNode) *chainView {
	//the mutex is intentionally not held since this is a constructor
	var c chainView
	c.setTip(tip)
	return &c
}

//genesis returns the genesis block for the chain view.this only differs
//from the exported version in that it is up to the caller to ensure the lock
//is held.
func (c *chainView) genesis() *blockNode {
	if len(c.nodes) == 0 {
		return nil
	}

	return c.nodes[0]
}

//genesis returns the genesis block for the chian view.
func (c *chainView) Genesis() *blockNode {
	if len(c.nodes) == 0 {
		return nil
	}

	return c.nodes[len(c.nodes)-1]

}

//tip reurns the current tip block node for the chain view.it will return nil
//if there is no tip.this only differs form the exproted version in that
//it is up to the caller to ensure the lock is held.
func (c *chainView)tip()*blockNode  {
	if len(c.nodes) == 0 {
		return nil
	}

	return c.nodes[len(c.nodes)-1]
}


//tip returns the current tip block node for the chain view.it will return
//nil if there is no tip.
func (c *chainView)Tip() *blockNode {
	c.mtx.Lock()
	tip := c.tip()
	c.mtx.Unlock()
	return tip
}


























