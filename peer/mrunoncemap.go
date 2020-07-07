package peer

import (
	"bytes"
	"container/list"
	"fmt"
	"sync"
)

//mrunoncemap provides a country safe map that is limited to a maximum
//number of items with eviction for the oldest entry when the limit is
//exceed.
type mruNonceMap struct {
	mtx       sync.Mutex
	nonceMap  map[uint64]*list.Element
	nonceList *list.List
	limit     uint
}

//string returns the map as a human-readable string
//this function is safe for concurent access
func (m *mruNonceMap) String() string {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	lastEntryNum := len(m.nonceMap) - 1
	curEntry := 0
	buf := bytes.NewBufferString("[")
	for nonce := range m.nonceMap {
		buf.WriteString(fmt.Sprintf("%d", nonce))
		if curEntry < lastEntryNum {
			buf.WriteString(",")
		}
		curEntry++
	}
	buf.WriteString("]")
	return fmt.Sprintf("<%d>%s", m.limit, buf.String())
}

//exists returns whether or not the passed nonce is in the map.
//this function is safe for concurent access.
func (m *mruNonceMap) Exists(nonce uint64) bool {
	m.mtx.Lock()
	_, exists := m.nonceMap[nonce]
	m.mtx.Unlock()
	return exists
}

//add the passed nonce to the map and handles eciction of the oldest item
//if adding the new item would exceed the max limit. Adding an existing item
//makes it the most recently used item.
func (m *mruNonceMap) Add(nonce uint64) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	//when the limit is zero .nothing can be added to the map .so just
	//return.
	if m.limit == 0 {
		return
	}

	//when the entry already exists move it to the front of the list
	//theryby marking it most receently used.
	if node, exists := m.nonceMap[nonce]; exists {
		m.nonceList.MoveToFront(node)
		return
	}

	//evict the least recently used entry (back of the list)if the new
	//entry would exceed the size limit for the map.aslo resure the list
	//node so a new one does have to be allocated.
	if uint(len(m.nonceMap))+1 > m.limit {
		node := m.nonceList.Back()
		lru := node.Value.(uint64)

		//evict least recently used item.
		delete(m.nonceMap, lru)

		//reuse the list node of the item that was just evicted for
		//the new item.
		node.Value = nonce
		m.nonceList.MoveToFront(node)
		m.nonceMap[nonce] = node
		return

	}

	//the limit has not been reached yet .so just add new item.
	node := m.nonceList.PushFront(nonce)
	m.nonceMap[nonce] = node

}

//delete the passed monce from the map(if it exists)
//this function is safe for concurrent access.
func (m *mruNonceMap) Delete(nonce uint64) {
	m.mtx.Lock()
	if node, exists := m.nonceMap[nonce]; exists {
		m.nonceList.Remove(node)
		delete(m.nonceMap, nonce)
	}
	m.mtx.Unlock()
}



// newMruNonceMap returns a new nonce map that is limited to the number of
// entries specified by limit.  When the number of entries exceeds the limit,
// the oldest (least recently used) entry will be removed to make room for the
// new entry.
func newMruNonceMap(limit uint) *mruNonceMap {
	m := mruNonceMap{
		nonceMap:  make(map[uint64]*list.Element),
		nonceList: list.New(),
		limit:     limit,
	}
	return &m
}


//over














