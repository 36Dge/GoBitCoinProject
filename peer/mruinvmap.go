package peer

import (
	"BtcoinProject/wire"
	"bytes"
	"container/list"
	"fmt"
	"sync"
)

//mruinventorymap provided a concurrency safe map that is limited to  maximum 
//number of items with eviction for the oldest entry when the limit is exceeded.
type mruInventoryMap struct {
	invMtx  sync.Mutex
	invMap  map[wire.InvVect]*list.Element
	invList *list.List
	limit   uint
}

//string returns the map as a human-readbales string
//this function if safe for concurent access
func (m *mruInventoryMap) String() string {
	m.invMtx.Lock()
	defer m.invMtx.Unlock()

	lastEntryNum := len(m.invMap) - 1
	curEntry := 0
	buf := bytes.NewBufferString("[")
	for iv := range m.invMap {
		buf.WriteString(fmt.Sprintf("%v", iv))
		if curEntry < lastEntryNum {
			buf.WriteString(",")
		}
		curEntry++
	}
	buf.WriteString("]")
	return fmt.Sprintf("<%d>%s", m.limit, buf.String())
}

//exists returns wherther or not passed inventory item is in the map
//this function is safe for concurent access.
func (m *mruInventoryMap) Exists(iv *wire.InvVect) bool {
	m.invMtx.Lock()
	_, exists := m.invMap[*iv]
	m.invMtx.Unlock()
	return exists
}

//adds the passed inventory to the map and handles eniction of the oldest
//item if adding the new item would exceed the max limit .adding an existing
//item makes it the most receently used item.
//this function is safe for concureent access.
func (m *mruInventoryMap) Add(iv *wire.InvVect) {
	m.invMtx.Lock()
	defer m.invMtx.Unlock()

	//when the limit is zero .nothing can be added to the map .so just return
	if m.limit == 0 {
		return
	}

	//when the entry already exists move it to the front of the list thereby
	//making it most recently used.
	if node, exists := m.invMap[*iv]; exists {
		m.invList.MoveToFront(node)
		return
	}

	//evict the least recently used entry (back of the list)if the new
	//entry would exceed the size limit for the map.aslo resure the list
	//node so a new one does not have to be allowed.
	if uint(len(m.invMap))+1 > m.limit {
		node := m.invList.Back()
		lru := node.Value.(*wire.InvVect)

		//evict least recently used item.
		delete(m.invMap, *lru)

		//reuse the list node of the item that was just evicted for the new item
		node.Value = iv
		m.invList.MoveToFront(node)
		m.invMap[*iv] = node
		return
	}

	//the limit has not been reached yet.so just add the new item.
	node := m.invList.PushFront(iv)
	m.invMap[*iv] = node

}

//delete the passed inventory item from the map(if it exists)
//this function is safe for concurent access
func (m *mruInventoryMap) Delete(iv *wire.InvVect) {
	m.invMtx.Lock()
	if node, exists := m.invMap[*iv]; exists {
		m.invList.Remove(node)
		delete(m.invMap, *iv)
	}
	m.invMtx.Unlock()
}

//returns a new inventory map that is limited to the number
//of entries specified by limit .when the number of entries excees
//the limit the oldest(least recently used)entry will be make
//room for the new entry
func newMruInventoryMap(limit uint) *mruInventoryMap {
	m := mruInventoryMap{
		invMap:  make(map[wire.InvVect]*list.Element),
		invList: list.New(),
		limit:   limit,
	}
	return &m
}


//over