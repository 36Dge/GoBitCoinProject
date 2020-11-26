package addrmgr

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"container/list"
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
	"time"
)

//addrmanager provides a concurrency safe address manager for caching potential peers on the bitcoin network.
type AddrManager struct {
	mtx            sync.Mutex
	peersFile      string
	lookupFunc     func(string) ([]net.IP, error)
	rand           *rand.Rand
	key            [32]byte
	addrIndex      map[string]*KnownAddress // address key to ka for all addrs.
	addrNew        [newBucketCount]map[string]*KnownAddress
	addrTried      [triedBucketCount]*list.List
	started        int32
	shutdown       int32
	wg             sync.WaitGroup
	quit           chan struct{}
	nTried         int
	nNew           int
	lamtx          sync.Mutex
	localAddresses map[string]*localAddress
	version        int
}

type serializedKnowAddress struct {
	Addr        string
	Src         string
	Attempts    int
	TimeStamp   int64
	LastAttempt int64
	LastSuccess int64
	Services    wire.ServiceFlag
	SrcServices wire.ServiceFlag
	// no refcount or tried, that is available from context.
}

type serializedAddrManager struct {
	Version      int
	Key          [32]byte
	Addresses    []*serializedKnownAddress
	NewBuckets   [newBucketCount][]string // string is NetAddressKey
	TriedBuckets [triedBucketCount][]string
}

type localAddress struct {
	na    *wire.NetAddress
	score AddressPriority
}

// AddressPriority type is used to describe the hierarchy of local address
// discovery methods.
type AddressPriority int

const (
	// InterfacePrio signifies the address is on a local interface
	InterfacePrio AddressPriority = iota

	// BoundPrio signifies the address has been explicitly bounded to.
	BoundPrio

	// UpnpPrio signifies the address was obtained from UPnP.
	UpnpPrio

	// HTTPPrio signifies the address was obtained from an external HTTP service.
	HTTPPrio

	// ManualPrio signifies the address was provided by --externalip.
	ManualPrio
)

const (
	// needAddressThreshold is the number of addresses under which the
	// address manager will claim to need more addresses.
	needAddressThreshold = 1000

	// dumpAddressInterval is the interval used to dump the address
	// cache to disk for future use.
	dumpAddressInterval = time.Minute * 10

	// triedBucketSize is the maximum number of addresses in each
	// tried address bucket.
	triedBucketSize = 256

	// triedBucketCount is the number of buckets we split tried
	// addresses over.
	triedBucketCount = 64

	// newBucketSize is the maximum number of addresses in each new address
	// bucket.
	newBucketSize = 64

	// newBucketCount is the number of buckets that we spread new addresses
	// over.
	newBucketCount = 1024

	// triedBucketsPerGroup is the number of tried buckets over which an
	// address group will be spread.
	triedBucketsPerGroup = 8

	// newBucketsPerGroup is the number of new buckets over which an
	// source address group will be spread.
	newBucketsPerGroup = 64

	// newBucketsPerAddress is the number of buckets a frequently seen new
	// address may end up in.
	newBucketsPerAddress = 8

	// numMissingDays is the number of days before which we assume an
	// address has vanished if we have not seen it announced  in that long.
	numMissingDays = 30

	// numRetries is the number of tried without a single success before
	// we assume an address is bad.
	numRetries = 3

	// maxFailures is the maximum number of failures we will accept without
	// a success before considering an address bad.
	maxFailures = 10

	// minBadDays is the number of days since the last success before we
	// will consider evicting an address.
	minBadDays = 7

	// getAddrMax is the most addresses that we will send in response
	// to a getAddr (in practise the most addresses we will return from a
	// call to AddressCache()).
	getAddrMax = 2500

	// getAddrPercent is the percentage of total addresses known that we
	// will share with a call to AddressCache.
	getAddrPercent = 23

	// serialisationVersion is the current version of the on-disk format.
	serialisationVersion = 2
)

//updateaddress is a helper fucntion to either update an address already konwn
//to the address manager.or to add the address if not already known.
func (a *AddrManager) updateAddress(netAddr, srcAddr *wire.NetAddress) {
	//faliter out non-routable address.note that non-routable also inculudes
	//invalid and local address.
	if !IsRoutable(netAddr) {
		return

	}

	addr := netAddressKey(netAddr)
	ka := a.find(netAddr)
	if ka != nil {
		//todo :only update addresses periodically
		//update the last seen time and services.
		//note that to prevent casuing excess garbage on getaddr
		//message the netaddresses in addrmanager are *immutable*
		//if we need to change them then we replace the pointer with a
		//new copy so that we do not have to copy every na for getaddr.
		if netAddr.Timestamp.After(ka.na.TimeStamp) ||
			(ka.na.Services&netAddr.Services) !=
				netAddr.Services {

			naCopy := *ka.na
			naCopy.Timestamp = netAddr.Timestamp
			naCopy.AddService(netAddr.Services)
			ka.na = &naCopy
		}

		//if already in tried.we have nothing to do here.
		if ka.tried {
			return
		}

		//already at out max?
		if ka.refs == newBucketsPerAddress {
			return
		}

		//the more entries we have .the less likely we are to add more.
		//likelihood is 2N.
		factor := int32(2 * ka.refs)
		if a.rand.Int31n(factor) != 0 {
			return
		}

	} else {
		//make a copy of the net address to avoied races since it is
		//updated elsewhere in the addremanager code and would otherwise
		//change the actual netaddress on the peer.
		netAddrCopy := *netAddr
		ka = &KnownAddress{na: &netAddrCopy, srcAddr: srcAddr}
		a.addrIndex[addr] = ka
		a.nNew++

	}

	bucket := a.getNewBucket(netAddr, srcAddr)

	//already exists?
	if _, ok := a.addrNew[bucket][addr]; ok {
		return
	}

	//enfore max address .
	if len(a.addrNew[bucket]) > newBucketSize {
		log.Tracef("new bucket is full ,expiring old")
		a.expireNew(bucket)

	}

	//add to new bucket
	ka.refs++
	a.addrNew[bucket][addr] = ka

	log.Tracef("added new address %s for a total of %d address ", addr, a.nTried+a.nNew)

}

//expirenew makes space in the new buckets by expiring the really bad entries.
//if no bad entries are available we loook at a new and remove the oldest.
func (a *AddrManager) expireNew(bucket int) {
	//first see if there are any entries that are so bad we can just throw
	//them away.otherwise we throw the oldest entry in the cache
	//bitcind here chooses four random and just throws the oldest of those away.
	//but we keep track of oldest in the initial traversal and use that
	//information instead.
	var oldest *KnownAddress
	for k, v := range a.addrNew[bucket] {
		if v.isBad() {
			log.Tracef("expiring bad address %v", k)
			delete(a.addrNew[bucket], k)
			v.refs--
			if v.refs == 0 {
			}
			a.nNew--
			delete(a.addrIndex, k)

		}
		continue

		if oldest == nil {
			oldest = v

		} else if !v.na.Timestamp.After(oldest.na.Timestamp) {
			oldest = v
		}
	}

	if oldest != nil{
		key := NetAddressKey{oldest.na}
		log.Tracef("expiring oldest address %v",key)

		delete(a.addrNew[bucket],key)
		oldest.refs--
		if oldest.refs == 0 {
			a.nNew--
			delete(a.addrIndex,key)
		}

	}





}

func (a *AddrManager) getNewBucket(netAddr, srcAddr *wire.NetAddress) int {
	// bitcoind:
	// doublesha256(key + sourcegroup + int64(doublesha256(key + group + sourcegroup))%bucket_per_source_group) % num_new_buckets

	data1 := []byte{}
	data1 = append(data1, a.key[:]...)
	data1 = append(data1, []byte(GroupKey(netAddr))...)
	data1 = append(data1, []byte(GroupKey(srcAddr))...)
	hash1 := chainhash.DoubleHashB(data1)
	hash64 := binary.LittleEndian.Uint64(hash1)
	hash64 %= newBucketsPerGroup
	var hashbuf [8]byte
	binary.LittleEndian.PutUint64(hashbuf[:], hash64)
	data2 := []byte{}
	data2 = append(data2, a.key[:]...)
	data2 = append(data2, GroupKey(srcAddr)...)
	data2 = append(data2, hashbuf[:]...)

	hash2 := chainhash.DoubleHashB(data2)
	return int(binary.LittleEndian.Uint64(hash2) % newBucketCount)
}
