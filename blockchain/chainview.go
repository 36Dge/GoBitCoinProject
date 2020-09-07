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
func (c *chainView) tip() *blockNode {
	if len(c.nodes) == 0 {
		return nil
	}

	return c.nodes[len(c.nodes)-1]
}

//tip returns the current tip block node for the chain view.it will return
//nil if there is no tip.
func (c *chainView) Tip() *blockNode {
	c.mtx.Lock()
	tip := c.tip()
	c.mtx.Unlock()
	return tip
}

//settip sets the chain view to use the provided block node as the
//current tip and ensure the view is consistent by populating it with
//the nodes obtained by walking backwards all the way to genesis block
//as necessary .further calls will only perform the mininum wordk need
//so swithing between chain tips is effceent this only differm form
//exported version in that it is up to caller to ensure the lock is held.
func (c *chainView) setTip(node *blockNode) {
	if node == nil {
		//keep the backing array around for potential futrue use.
		c.nodes = c.nodes[:0]
		return
	}

	//crete or resize the slice that will hold the block nodes to the
	//provided tip height.when creating the slice.it is created with
	//some additional capacity for the underlying arary as append wuold
	//do in order to reduce overhead when extending the chain later.
	//as long as the underlying array hash enough capacity.simply expand or
	//contract the slice accordingly.the additional capacity is chosen
	//such that the array should only have to be extended about once a
	//week.
	needed := node.height + 1
	if int32(cap(c.nodes)) < needed {
		nodes := make([]*blockNode, needed, needed+approxNodeSPerWeek)
		copy(nodes, c.nodes)
		c.nodes = nodes
	} else {
		prevLen := int32(len(c.nodes))
		c.nodes = c.nodes[0:needed]
		for i := prevLen; i < needed; i++ {
			c.nodes[i] = nil
		}
	}

	for node != nil && c.nodes[node.height] != node {
		c.nodes[node.height] = node
		node = node.parent
	}

}

// SetTip sets the chain view to use the provided block node as the current tip
// and ensures the view is consistent by populating it with the nodes obtained
// by walking backwards all the way to genesis block as necessary.  Further
// calls will only perform the minimum work needed, so switching between chain
// tips is efficient.
//
// This function is safe for concurrent access.
func (c *chainView) SetTip(node *blockNode) {
	c.mtx.Lock()
	c.setTip(node)
	c.mtx.Unlock()
}

// height returns the height of the tip of the chain view.  It will return -1 if
// there is no tip (which only happens if the chain view has not been
// initialized).  This only differs from the exported version in that it is up
// to the caller to ensure the lock is held.
//
// This function MUST be called with the view mutex locked (for reads).
func (c *chainView) height() int32 {
	return int32(len(c.nodes) - 1)
}

// Height returns the height of the tip of the chain view.  It will return -1 if
// there is no tip (which only happens if the chain view has not been
// initialized).
//
// This function is safe for concurrent access.
func (c *chainView) Height() int32 {
	c.mtx.Lock()
	height := c.height()
	c.mtx.Unlock()
	return height
}

//nodebyheight returns the block node at the specifed height ,nil will be
//returned if the height does not exit ,this only differm form the exproted
//version in that it is up to the caller to ensure the lock is held.
func (c *chainView)nodeByHeight(height int32)*blockNode  {
	if height < 0 || height >= int32((len(c.nodes))){
		return nil
	}

	return c.nodes[height]
}

// NodeByHeight returns the block node at the specified height.  Nil will be
// returned if the height does not exist.
//
// This function is safe for concurrent access.
func (c *chainView) NodeByHeight(height int32) *blockNode {
	c.mtx.Lock()
	node := c.nodeByHeight(height)
	c.mtx.Unlock()
	return node
}

//equals returns whether or not two chain views are the same .uninitialized
//views (tip set to nil)are considered euqal.
func (c *chainView)Equals(other *chainView)bool  {
	c.mtx.Lock()
	other.mtx.Lock()
	equals := len(c.nodes) == len(other.nodes) && c.tip() == other.tip()
	other.mtx.Unlock()
	c.mtx.Unlock()
	return equals
}

// contains returns whether or not the chain view contains the passed block
// node.  This only differs from the exported version in that it is up to the
// caller to ensure the lock is held.
//
// This function MUST be called with the view mutex locked (for reads).
func (c *chainView) contains(node *blockNode) bool {
	return c.nodeByHeight(node.height) == node
}

// Contains returns whether or not the chain view contains the passed block
// node.
//
// This function is safe for concurrent access.
func (c *chainView) Contains(node *blockNode) bool {
	c.mtx.Lock()
	contains := c.contains(node)
	c.mtx.Unlock()
	return contains
}




























