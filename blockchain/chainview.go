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
func (c *chainView) nodeByHeight(height int32) *blockNode {
	if height < 0 || height >= int32((len(c.nodes))) {
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
func (c *chainView) Equals(other *chainView) bool {
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

// next returns the successor to the provided node for the chain view.  It will
// return nil if there is no successor or the provided node is not part of the
// view.  This only differs from the exported version in that it is up to the
// caller to ensure the lock is held.
//
// See the comment on the exported function for more details.
//
// This function MUST be called with the view mutex locked (for reads).
func (c *chainView) next(node *blockNode) *blockNode {
	if node == nil || !c.contains(node) {
		return nil
	}

	return c.nodeByHeight(node.height + 1)
}

// Next returns the successor to the provided node for the chain view.  It will
// return nil if there is no successfor or the provided node is not part of the
// view.
//
// For example, assume a block chain with a side chain as depicted below:
//   genesis -> 1 -> 2 -> 3 -> 4  -> 5 ->  6  -> 7  -> 8
//                         \-> 4a -> 5a -> 6a
//
// Further, assume the view is for the longer chain depicted above.  That is to
// say it consists of:
//   genesis -> 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8
//
// Invoking this function with block node 5 would return block node 6 while
// invoking it with block node 5a would return nil since that node is not part
// of the view.
//
// This function is safe for concurrent access.
func (c *chainView) Next(node *blockNode) *blockNode {
	c.mtx.Lock()
	next := c.next(node)
	c.mtx.Unlock()
	return next
}

//findForm returns the final common block betweent the provided node and
//the chain view.it will return nil if there is no common block,this onl
//differs form the exproted version in that it is up to the caller to enssure
//the lock is held.
//see the exported findfork comments for more detaisl
func (c *chainView) findFork(node *blockNode) *blockNode {
	//no fork point for node that doesn,t exist.
	if node == nil {
		return nil
	}

	//when the height of the passed node is higher than the height of
	//the tip of the current chain view.walk backwards throght the nodes of
	//the other chain until the

	// which case there is no common node between the two).
	//
	// NOTE: This isn't strictly necessary as the following section will
	// find the node as well, however, it is more efficient to avoid the
	// contains check since it is already known that the common node can't
	// possibly be past the end of the current chain view.  It also allows
	// this code to take advantage of any potential future optimizations to
	// the Ancestor function such as using an O(log n) skip list.

	chainHeight := c.height()
	if node.height > chainHeight {
		node = node.Ancestor(chainHeight)
	}

	//walk the other chain backwards as long as the current one does not
	//contain the node or there are no more nodes in which case there is
	//no common node between the two
	for node != nil && !c.contains(node) {
		node = node.parent
	}

	return node

}

// FindFork returns the final common block between the provided node and the
// the chain view.  It will return nil if there is no common block.
//
// For example, assume a block chain with a side chain as depicted below:
//   genesis -> 1 -> 2 -> ... -> 5 -> 6  -> 7  -> 8
//                                \-> 6a -> 7a
//
// Further, assume the view is for the longer chain depicted above.  That is to
// say it consists of:
//   genesis -> 1 -> 2 -> ... -> 5 -> 6 -> 7 -> 8.
//
// Invoking this function with block node 7a would return block node 5 while
// invoking it with block node 7 would return itself since it is already part of
// the branch formed by the view.
//
// This function is safe for concurrent access.
func (c *chainView) FindFork(node *blockNode) *blockNode {
	c.mtx.Lock()
	fork := c.findFork(node)
	c.mtx.Unlock()
	return fork
}

//blocklocator returns a block locator for the passed blok node ,the passed
//node can be nil in which case the block locator for the current tip
//associated with the view be returned ,this only differs form the exported
//version in that it is up to the caller to ensure the lock is held .
//see the exported BlockLocator function comments for more details.
// This function MUST be called with the view mutex locked (for reads).
func (c *chainView) blockLocator(node *blockNode) BlockLocator {
	//use the current tip if requested.
	if node == nil {
		node = c.tip()
	}
	if node == nil {
		return nil
	}

	//calculate the max number of entries that will ultimately be in the
	//block locator ,see the description of the algorithm for how these
	//numbers are derived.
	var maxEntries uint8

	if node.height <= 12 {
		maxEntries = uint8(node.height) + 1
	} else {
		//requested hash iteself + previous 10 entries + genesis block .
		//then floor(log2(height - 10)entries for the skip protion
		adjustedHeight := uint32(node.height) - 10
		maxEntries = 12 + fastLog2Floor(adjustedHeight)

	}
	locator := make(BlockLocator, 0, maxEntries)

	step := int32(1)
	for node != nil {
		locator = append(locator, &node.hash)

		//nothing more to add once the genesis block has been added.
		if node.height == 0 {
			break
		}

		// Calculate height of previous node to include ensuring the
		// final node is the genesis block.
		height := node.height - step
		if height < 0 {
			height = 0
		}

		//when the node is in the current chain view. all of its ancestors
		//must be too.so use a much faseter lookun in that case otherwise
		//fall back to walking backwards through the nodes of the other chain
		//to the corrent ancestor.
		if c.contains(node) {
			node = c.nodes[height]
		} else {
			node = node.Ancestor(height)

		}

		//once 11 entries have been included. start doubling the distance betwween
		//inclued hashes.
		if len(locator) > 10 {
			step *= 2
		}

	}

	return locator
}


// BlockLocator returns a block locator for the passed block node.  The passed
// node can be nil in which case the block locator for the current tip
// associated with the view will be returned.
//
// See the BlockLocator type for details on the algorithm used to create a block
// locator.
//
// This function is safe for concurrent access.
func (c *chainView) BlockLocator(node *blockNode) BlockLocator {
	c.mtx.Lock()
	locator := c.blockLocator(node)
	c.mtx.Unlock()
	return locator
}



//over





