package peer

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"bytes"
	"container/list"
	"context"
	"fmt"
	"github.com/btcsuite/go-socks/socks"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/proxy"
	"golang.org/x/text/cases"
	"io"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	//maxprotocolversion is the max protocol version the peer supports
	MaxProtocolVersion = wire.FeeFilterVersion

	//defaulttrickleinterval is the min time between attempts to send an
	//inv message to a peer
	DefaultTrickleInterval = 10 * time.Second

	//minacceptable protocolversion is the lowest protocol version that a
	//connected peer may support.
	MinAcceptableProtocolVersion = wire.MultipleAddressVersion

	//outputbuffersize is the number of elements the output channel use.
	outputBufferSize = 50

	//invTricklesize is the maximum amount of invetory to send in a single
	//message when trickling inventory to remote peers.
	maxInvTrickleSize = 1000

	//maxknowninventroy is the maximum number of items to keep in the known
	//inventory cache.
	maxKnownTnventory = 1000

	//pinginterval is the interval of time to wait in between sending ping
	//message.
	pingInterval = 2 * time.Second

	//negotiatetimeout is the duration of inactivity before we timeout a
	//peer that has not completed the initial version negotiation.
	negotiateTimeout = 30 * time.Second

	//idleTimeout is the duration of inactivity before we time out a peer
	idleTimeout = 5 * time.Minute

	//stalltickinterval is the interval of time between each check for stalled
	//peers.
	stallTrickTinterval = 15 * time.Second

	//stallresponsetimeout is base base maximum amount of time messages that
	// expect a response will wait before disconnecting the peer for
	// stalling.  The deadlines are adjusted for callback running times and
	// only checked on each stall tick interval.

	stallReponseTimeout = 30 * time.Second
)

var (

	//nodecount is the total number of peer connections made since starup
	//and is used to assign id to a peer.
	nodeCount int32

	//zerohash is the zero value hash (all zeros) it is defineds as a conbenience
	zeroHash chainhash.Hash

	//sentnonces houses the unique nonces that are gennerated when pushing
	//version message that are used to detect self connection.s
	sentNonces = newMruNonceMap(50)

	// allowSelfConns is only used to allow the tests to bypass the self
	// connection detecting and disconnect logic since they intentionally
	// do so for testing purposes.
	allowSelfConns bool
)

// MessageListeners defines callback function pointers to invoke with message
// listeners for a peer. Any listener which is not set to a concrete callback
// during peer initialization is ignored. Execution of multiple message
// listeners occurs serially, so one callback blocks the execution of the next.
//
// NOTE: Unless otherwise documented, these listeners must NOT directly call any
// blocking calls (such as WaitForShutdown) on the peer instance since the input
// handler goroutine blocks until the callback has completed.  Doing so will
// result in a deadlock.
type MessageListeners struct {
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnGetAddr func(p *Peer, msg *wire.MsgGetAddr)

	// OnAddr is invoked when a peer receives an addr bitcoin message.
	OnAddr func(p *Peer, msg *wire.MsgAddr)

	// OnPing is invoked when a peer receives a ping bitcoin message.
	OnPing func(p *Peer, msg *wire.MsgPing)

	// OnPong is invoked when a peer receives a pong bitcoin message.
	OnPong func(p *Peer, msg *wire.MsgPong)

	// OnAlert is invoked when a peer receives an alert bitcoin message.
	OnAlert func(p *Peer, msg *wire.MsgAlert)

	// OnMemPool is invoked when a peer receives a mempool bitcoin message.
	OnMemPool func(p *Peer, msg *wire.MsgMemPool)

	// OnTx is invoked when a peer receives a tx bitcoin message.
	OnTx func(p *Peer, msg *wire.MsgTx)

	// OnBlock is invoked when a peer receives a block bitcoin message.
	OnBlock func(p *Peer, msg *wire.MsgBlock, buf []byte)

	// OnCFilter is invoked when a peer receives a cfilter bitcoin message.
	OnCFilter func(p *Peer, msg *wire.MsgCFilter)

	// OnCFHeaders is invoked when a peer receives a cfheaders bitcoin
	// message.
	OnCFHeaders func(p *Peer, msg *wire.MsgCFHeaders)

	// OnCFCheckpt is invoked when a peer receives a cfcheckpt bitcoin
	// message.
	OnCFCheckpt func(p *Peer, msg *wire.MsgCFCheckpt)

	// OnInv is invoked when a peer receives an inv bitcoin message.
	OnInv func(p *Peer, msg *wire.MsgInv)

	// OnHeaders is invoked when a peer receives a headers bitcoin message.
	OnHeaders func(p *Peer, msg *wire.MsgHeaders)

	// OnNotFound is invoked when a peer receives a notfound bitcoin
	// message.
	OnNotFound func(p *Peer, msg *wire.MsgNotFound)

	// OnGetData is invoked when a peer receives a getdata bitcoin message.
	OnGetData func(p *Peer, msg *wire.MsgGetData)

	// OnGetBlocks is invoked when a peer receives a getblocks bitcoin
	// message.
	OnGetBlocks func(p *Peer, msg *wire.MsgGetBlocks)

	// OnGetHeaders is invoked when a peer receives a getheaders bitcoin
	// message.
	OnGetHeaders func(p *Peer, msg *wire.MsgGetHeaders)

	// OnGetCFilters is invoked when a peer receives a getcfilters bitcoin
	// message.
	OnGetCFilters func(p *Peer, msg *wire.MsgGetCFilters)

	// OnGetCFHeaders is invoked when a peer receives a getcfheaders
	// bitcoin message.
	OnGetCFHeaders func(p *Peer, msg *wire.MsgGetCFHeaders)

	// OnGetCFCheckpt is invoked when a peer receives a getcfcheckpt
	// bitcoin message.
	OnGetCFCheckpt func(p *Peer, msg *wire.MsgGetCFCheckpt)

	// OnFeeFilter is invoked when a peer receives a feefilter bitcoin message.
	OnFeeFilter func(p *Peer, msg *wire.MsgFeeFilter)

	// OnFilterAdd is invoked when a peer receives a filteradd bitcoin message.
	OnFilterAdd func(p *Peer, msg *wire.MsgFilterAdd)

	// OnFilterClear is invoked when a peer receives a filterclear bitcoin
	// message.
	OnFilterClear func(p *Peer, msg *wire.MsgFilterClear)

	// OnFilterLoad is invoked when a peer receives a filterload bitcoin
	// message.
	OnFilterLoad func(p *Peer, msg *wire.MsgFilterLoad)

	// OnMerkleBlock  is invoked when a peer receives a merkleblock bitcoin
	// message.
	OnMerkleBlock func(p *Peer, msg *wire.MsgMerkleBlock)

	// OnVersion is invoked when a peer receives a version bitcoin message.
	// The caller may return a reject message in which case the message will
	// be sent to the peer and the peer will be disconnected.
	OnVersion func(p *Peer, msg *wire.MsgVersion) *wire.MsgReject

	// OnVerAck is invoked when a peer receives a verack bitcoin message.
	OnVerAck func(p *Peer, msg *wire.MsgVerAck)

	// OnReject is invoked when a peer receives a reject bitcoin message.
	OnReject func(p *Peer, msg *wire.MsgReject)

	// OnSendHeaders is invoked when a peer receives a sendheaders bitcoin
	// message.
	OnSendHeaders func(p *Peer, msg *wire.MsgSendHeaders)

	// OnRead is invoked when a peer receives a bitcoin message.  It
	// consists of the number of bytes read, the message, and whether or not
	// an error in the read occurred.  Typically, callers will opt to use
	// the callbacks for the specific message types, however this can be
	// useful for circumstances such as keeping track of server-wide byte
	// counts or working with custom message types for which the peer does
	// not directly provide a callback.
	OnRead func(p *Peer, bytesRead int, msg wire.Message, err error)

	// OnWrite is invoked when we write a bitcoin message to a peer.  It
	// consists of the number of bytes written, the message, and whether or
	// not an error in the write occurred.  This can be useful for
	// circumstances such as keeping track of server-wide byte counts.
	OnWrite func(p *Peer, bytesWritten int, msg wire.Message, err error)
}

//config is the struct to hold configuration options useful to peer
type Config struct {
	//newestblock specifies a callback which provides the newest block
	//datails to peer as needed .this can be nil in which case the peer
	//will report a block height of 0.however it is good practice for
	//peers to specify this so their curenttly best known is accrtaely reported
	NewwestBlco HashFunc

	//hosttonetaddress returns the netaddres for the given host.this can
	//be nil in which case the host will be parsed as an ip address
	HostToNetAddress HostToNetAddrFunc

	//proxy indicates a porxy is being used for connections the only
	//effect this has is to prevent leaking the tor proxy addres .so ti
	//only needs to specified if using a tor proxy.
	Proxy string

	//useragentnaem specifies the user agent name to adversie .it is
	//highly recommanded to specify this value.
	UserAgentName string

	//useagentversion specifies the user agent version to advertise .it
	//is hinghly recommanded to specify this vulue and that it follows
	//the form"major.minor.revison"g.
	UserAgentVersion string

	//useragentcommnets specify the user agent comments to advertise
	//these values must not contain the illegal characters specifed in bip 14:
	UserAgentComment []string

	//chainParams identifies which chain parameters the peer is associated
	//with.it is highly recommended to specify this field .however it can be
	//ommited in which case the test newwork will be used.
	ChainParams *chaincfg.Params

	//services specifies which services to advertise as supported by the
	//local peer.this field can be ommited in which case it will be o - and
	//therefore advertise no suported services.
	Services wire.ServiceFlag

	//protocolversion specifies the maximum protocol version to use and
	//advertise this field can be ommited in which case peer.maxprotocolversion
	//will be used .
	ProtocolVersion uint32

	//listenners houses callback functions to be invoked on receiving
	//peer message.
	Listeners MessageListeners

	//trickintervel is the duration of the tircker which trickles down
	//the invnetval to a peer
	TrickleInterval time.Duration
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

//newnetaddress attempts to extract the ip address and port from th e
//passed net.addr interface and create a bitcoin netaddress structre using
//that information.
func newNetAddress(addr net.Addr, services wire.ServiceFlag) (*wire.NetAddress, error) {
	//addr will be a net.tcpaddr when not using a proxy .
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		ip := tcpAddr.IP
		port := uint16(tcpAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	//addr will be a sock.proxiedaddr when using a proxy
	if proxiedAddr, ok := addr.(*socks.ProxiedAddr); ok {
		ip := net.ParseIP(proxiedAddr.Host)
		if ip == nil {
			ip = net.ParseIP("0.0.0.0")
		}
		port := uint16(proxiedAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	//for the most part ,addr should be one of the two above cases,but
	//to be safe,fall back to trying to parse the information from the
	//address string as a last resort.
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	na := wire.NewNetAddressIPPort(ip, uint16(port), services)
	return na, nil

}

//outmsg is used to house a message to be sent along with a channel to singal
//when the message has been sent(or will not be sent due to things such as shutdown)
type outMsg struct {
	msg      wire.Message
	doneChan chan<- struct{}
	encoding wire.MessageEncoding
}

//stallcontrolcmd represents the command of a stall control message.
type stallControlCmd uint8

//constants for the command of a stall control message.
const (

	// sccSendMessage indicates a message is being sent to the remote peer.
	sccSendMessage stallControlCmd = iota

	// sccReceiveMessage indicates a message has been received from the
	// remote peer.
	sccReceiveMessage

	// sccHandlerStart indicates a callback handler is about to be invoked.
	sccHandlerStart

	// sccHandlerStart indicates a callback handler has completed.
	sccHandlerDone
)

//stallcontrolmsg is used to signal the stall handler about specific enents
//so it can porperly detect and handle stalled remote peers.
type stallControlMsg struct {
	command stallControlCmd
	message wire.Message
}

//startsnap is a snapshot of peer starts at a point in time
type StartsSnap struct {
	ID             int32
	Addr           string
	Services       wire.ServiceFlag
	LastSend       time.Time
	LastRecv       time.Time
	BytesSent      uint64
	BytesRecv      uint64
	ConnTime       time.Time
	TimeOffset     int64
	Version        uint32
	UserAgent      string
	Inbound        bool
	StartingHeight int32
	LastBlock      int32
	LastPingNonce  uint64
	LastPingTime   time.Time
	LastPingMicros int64
}

//hashfunc is function which returns a block hash,height and error
//it is used as a callback to get newest block details.
type HashFunc func() (hash *chainhash.Hash, height int32, err error)

//addrfunc is a func which takes an address and returns a related address
type AddrFunc func(remoteAddr *wire.NetAddress) *wire.NetAddress

//hosttonetaddrfunc is a func which takes a host ,port ,services and returns
//the netaddress.
type HostToNetAddrFunc func(host string, port uint16, services wire.ServiceFlag) (*wire.NetAddress, error)

// NOTE: The overall data flow of a peer is split into 3 goroutines.  Inbound
// messages are read via the inHandler goroutine and generally dispatched to
// their own handler.  For inbound data-related messages such as blocks,
// transactions, and inventory, the data is handled by the corresponding
// message handlers.  The data flow for outbound messages is split into 2
// goroutines, queueHandler and outHandler.  The first, queueHandler, is used
// as a way for external entities to queue messages, by way of the QueueMessage
// function, quickly regardless of whether the peer is currently sending or not.
// It acts as the traffic cop between the external world and the actual
// goroutine which writes to the network socket.

// Peer provides a basic concurrent safe bitcoin peer for handling bitcoin
// communications via the peer-to-peer protocol.  It provides full duplex
// reading and writing, automatic handling of the initial handshake process,
// querying of usage statistics and other information about the remote peer such
// as its address, user agent, and protocol version, output message queuing,
// inventory trickling, and the ability to dynamically register and unregister
// callbacks for handling bitcoin protocol messages.
//
// Outbound messages are typically queued via QueueMessage or QueueInventory.
// QueueMessage is intended for all messages, including responses to data such
// as blocks and transactions.  QueueInventory, on the other hand, is only
// intended for relaying inventory as it employs a trickling mechanism to batch
// the inventory together.  However, some helper functions for pushing messages
// of specific types that typically require common special handling are
// provided as a convenience.
type Peer struct {
	// The following variables must only be used atomically.
	bytesReceived uint64
	bytesSent     uint64
	lastRecv      int64
	lastSend      int64
	connected     int32
	disconnect    int32

	conn net.Conn

	// These fields are set at creation time and never modified, so they are
	// safe to read from concurrently without a mutex.
	addr    string
	cfg     Config
	inbound bool

	flagsMtx             sync.Mutex // protects the peer flags below
	na                   *wire.NetAddress
	id                   int32
	userAgent            string
	services             wire.ServiceFlag
	versionKnown         bool
	advertisedProtoVer   uint32 // protocol version advertised by remote
	protocolVersion      uint32 // negotiated protocol version
	sendHeadersPreferred bool   // peer sent a sendheaders message
	verAckReceived       bool
	witnessEnabled       bool

	wireEncoding wire.MessageEncoding

	knownInventory     *mruInventoryMap
	prevGetBlocksMtx   sync.Mutex
	prevGetBlocksBegin *chainhash.Hash
	prevGetBlocksStop  *chainhash.Hash
	prevGetHdrsMtx     sync.Mutex
	prevGetHdrsBegin   *chainhash.Hash
	prevGetHdrsStop    *chainhash.Hash

	// These fields keep track of statistics for the peer and are protected
	// by the statsMtx mutex.
	statsMtx           sync.RWMutex
	timeOffset         int64
	timeConnected      time.Time
	startingHeight     int32
	lastBlock          int32
	lastAnnouncedBlock *chainhash.Hash
	lastPingNonce      uint64    // Set to nonce if we have a pending ping.
	lastPingTime       time.Time // Time we sent last ping.
	lastPingMicros     int64     // Time for last ping to return.

	stallControl  chan stallControlMsg
	outputQueue   chan outMsg
	sendQueue     chan outMsg
	sendDoneQueue chan struct{}
	outputInvChan chan *wire.InvVect
	inQuit        chan struct{}
	queueQuit     chan struct{}
	outQuit       chan struct{}
	quit          chan struct{}
}

//string retuns the peer address and directionality as a human-readble string
func (p *Peer) String() string {
	return fmt.Sprintf("%s(%s)", p.addr, directionString(p.inbound))
}

//updatelastblockheight upadates the last known blcok for the peer.
func (p *Peer) UpdateLastBlockHeight(newHeight int32) {
	p.statsMtx.Lock()
	log.Tracef("updating last block height of peer %v from %v to %v", p.addr, p.lastBlock, newHeight)
	p.lastBlock = newHeight
	p.statsMtx.Unlock()
}

//updateslastannouncemedblock updates meta_data about the last block hash this
//peer is known to have annouced.

func (p *Peer) UpdateLastAnnouceBlock(blkHash *chainhash.Hash) {
	log.Tracef("updating last blk for peer %v,%v", p.addr, blkHash)

	p.statsMtx.Lock()
	p.lastAnnouncedBlock = blkHash
	p.statsMtx.Unlock()

}

//addknowninvetnro adds the passed inventroy to the cache of konown inbventro
//for the peer.
func (p *Peer) AddKnownInventory(invVect *wire.InvVect) {
	p.knownInventory.Add(invVect)

}

//startssnapshot returns a snapshot of the current peer flags and statics
func (p *Peer) StatsSnapshot() *StartsSnap {
	p.statsMtx.RLock()

	p.flagsMtx.Lock()
	id := p.id
	addr := p.addr
	userAgent := p.userAgent
	services := p.services
	protocolVersion := p.advertisedProtoVer
	p.flagsMtx.Unlock()

	//get a copy of all relevant flags and stats.
	statsSnap := &StartsSnap{
		ID:             id,
		Addr:           addr,
		UserAgent:      userAgent,
		Services:       services,
		LastSend:       p.LastSend(),
		LastRecv:       p.LastRecv(),
		BytesSent:      p.BytesSent(),
		BytesRecv:      p.BytesReceived(),
		ConnTime:       p.timeConnected,
		TimeOffset:     p.timeOffset,
		Version:        protocolVersion,
		Inbound:        p.inbound,
		StartingHeight: p.startingHeight,
		LastBlock:      p.lastBlock,
		LastPingNonce:  p.lastPingNonce,
		LastPingMicros: p.lastPingMicros,
		LastPingTime:   p.lastPingTime,
	}
	p.statsMtx.RUnlock()
	return statsSnap
}

//id returns the peer id
func (p *Peer) ID() int32 {
	p.flagsMtx.Lock()
	id := p.id
	p.flagsMtx.Unlock()

	return id
}

//na returns the peer network address.
func (p *Peer) NA() *wire.NetAddress {
	p.flagsMtx.Lock()
	na := p.na
	p.flagsMtx.Unlock()

	return na
}

//addr returns the peer address
func (p *Peer) Addr() string {
	//the address does not change after initialization .therefore it is
	//not protected by a mutex
	return p.addr
}

//inbound returns whether the peer is inbound
func (p *Peer) Inbound() bool {
	return p.inbound
}

// Services returns the services flag of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) Services() wire.ServiceFlag {
	p.flagsMtx.Lock()
	services := p.services
	p.flagsMtx.Unlock()

	return services
}

// UserAgent returns the user agent of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) UserAgent() string {
	p.flagsMtx.Lock()
	userAgent := p.userAgent
	p.flagsMtx.Unlock()

	return userAgent
}

// LastAnnouncedBlock returns the last announced block of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastAnnouncedBlock() *chainhash.Hash {
	p.statsMtx.RLock()
	lastAnnouncedBlock := p.lastAnnouncedBlock
	p.statsMtx.RUnlock()

	return lastAnnouncedBlock
}

// LastPingNonce returns the last ping nonce of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastPingNonce() uint64 {
	p.statsMtx.RLock()
	lastPingNonce := p.lastPingNonce
	p.statsMtx.RUnlock()

	return lastPingNonce
}

// LastPingTime returns the last ping time of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastPingTime() time.Time {
	p.statsMtx.RLock()
	lastPingTime := p.lastPingTime
	p.statsMtx.RUnlock()

	return lastPingTime
}

// LastPingMicros returns the last ping micros of the remote peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastPingMicros() int64 {
	p.statsMtx.RLock()
	lastPingMicros := p.lastPingMicros
	p.statsMtx.RUnlock()

	return lastPingMicros
}

// VersionKnown returns the whether or not the version of a peer is known
// locally.
//
// This function is safe for concurrent access.
func (p *Peer) VersionKnown() bool {
	p.flagsMtx.Lock()
	versionKnown := p.versionKnown
	p.flagsMtx.Unlock()

	return versionKnown
}

// VerAckReceived returns whether or not a verack message was received by the
// peer.
//
// This function is safe for concurrent access.
func (p *Peer) VerAckReceived() bool {
	p.flagsMtx.Lock()
	verAckReceived := p.verAckReceived
	p.flagsMtx.Unlock()

	return verAckReceived
}

// ProtocolVersion returns the negotiated peer protocol version.
//
// This function is safe for concurrent access.
func (p *Peer) ProtocolVersion() uint32 {
	p.flagsMtx.Lock()
	protocolVersion := p.protocolVersion
	p.flagsMtx.Unlock()

	return protocolVersion
}

// LastBlock returns the last block of the peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastBlock() int32 {
	p.statsMtx.RLock()
	lastBlock := p.lastBlock
	p.statsMtx.RUnlock()

	return lastBlock
}

// LastSend returns the last send time of the peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastSend() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastSend), 0)
}

// LastRecv returns the last recv time of the peer.
//
// This function is safe for concurrent access.
func (p *Peer) LastRecv() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastRecv), 0)
}

// LocalAddr returns the local address of the connection.
//
// This function is safe fo concurrent access.
func (p *Peer) LocalAddr() net.Addr {
	var localAddr net.Addr
	if atomic.LoadInt32(&p.connected) != 0 {
		localAddr = p.conn.LocalAddr()
	}
	return localAddr
}

// BytesSent returns the total number of bytes sent by the peer.
//
// This function is safe for concurrent access.
func (p *Peer) BytesSent() uint64 {
	return atomic.LoadUint64(&p.bytesSent)
}

// BytesReceived returns the total number of bytes received by the peer.
//
// This function is safe for concurrent access.
func (p *Peer) BytesReceived() uint64 {
	return atomic.LoadUint64(&p.bytesReceived)
}

// TimeConnected returns the time at which the peer connected.
//
// This function is safe for concurrent access.
func (p *Peer) TimeConnected() time.Time {
	p.statsMtx.RLock()
	timeConnected := p.timeConnected
	p.statsMtx.RUnlock()

	return timeConnected
}

// TimeOffset returns the number of seconds the local time was offset from the
// time the peer reported during the initial negotiation phase.  Negative values
// indicate the remote peer's time is before the local time.
//
// This function is safe for concurrent access.
func (p *Peer) TimeOffset() int64 {
	p.statsMtx.RLock()
	timeOffset := p.timeOffset
	p.statsMtx.RUnlock()

	return timeOffset
}

// StartingHeight returns the last known height the peer reported during the
// initial negotiation phase.
//
// This function is safe for concurrent access.
func (p *Peer) StartingHeight() int32 {
	p.statsMtx.RLock()
	startingHeight := p.startingHeight
	p.statsMtx.RUnlock()

	return startingHeight
}

// WantsHeaders returns if the peer wants header messages instead of
// inventory vectors for blocks.
//
// This function is safe for concurrent access.
func (p *Peer) WantsHeaders() bool {
	p.flagsMtx.Lock()
	sendHeadersPreferred := p.sendHeadersPreferred
	p.flagsMtx.Unlock()

	return sendHeadersPreferred
}

// IsWitnessEnabled returns true if the peer has signalled that it supports
// segregated witness.
//
// This function is safe for concurrent access.
func (p *Peer) IsWitnessEnabled() bool {
	p.flagsMtx.Lock()
	witnessEnabled := p.witnessEnabled
	p.flagsMtx.Unlock()

	return witnessEnabled
}

//pushaddrmsg sends an addr messge to the connected peer using the provied
//address. this function is useful over manually sending the message via
//queuemessage since it atuomaticllly limits the address to the maximum
//number allowed by the message and randomizes the chosen address when
//there are too many. it return the address that were actully sent and
//no message will be sent if there are no entries in the porvided address
//sliece.
func (p *Peer) PushAddrMsg(address []*wire.NetAddress) ([]*wire.NetAddress, error) {
	addressCount := len(address)

	//nothing to send
	if addressCount == 0 {
		return nil, nil
	}

	msg := wire.NewMsgAddr()
	msg.AddrList = make([]*wire.NetAddress, addressCount)
	copy(msg.AddrList, address)

	//randomize the address sent if there are more than the maximum assloed.
	if addressCount > wire.MaxAddrPerMsg {
		//shuffle the address list
		for i := 0; i < wire.MaxInvPerMsg; i++ {
			j := i + rand.Intn(addressCount-1)
			msg.AddrList[i], msg.AddrList[j] = msg.AddrList[j], msg.AddrList[i]
		}

		//truncate it to the maximum size
		msg.AddrList = msg.AddrList[:wire.MaxInvPerMsg]
	}

	p.QueueMessage(msg, nil)
	return msg.AddrList, nil
}

//pushgetblockmsg sends a getblocks message for teh provided block
//locator and stop hash. it will ingnore back-to-back duplicate requests.
func (p *Peer) PushGetBlocksMsg(locator blockchain.BlockLocator, stopHash *chainhash.Hash) error {
	//extract the begin hash from the block loactor,if one was specified .
	//to use for filtering duplicate getblocks requests
	var beginHash *chainhash.Hash
	if len(locator) > 0 {
		beginHash = locator[0]
	}

	//fileter duplicate getblocks requests
	p.prevGetBlocksMtx.Lock()
	isDuplicate := p.prevGetBlocksStop != nil && p.prevGetBlocksBegin != nil &&
		beginHash != nil && stopHash.IsEqual(p.prevGetBlocksStop) && beginHash.IsEqual(p.prevGetBlocksBegin)
	p.prevGetBlocksMtx.Unlock()

	if isDuplicate {
		log.Tracef("filtering duplicate [getblocks]with begin "+
			"hash %v,stop hash %v", beginHash, stopHash)
		return nil
	}

	//construct the getblocks request and quenu it to be sent.
	msg := wire.NewMsgGetBlocks(stopHash)
	for _, hash := range locator {
		err := msg.AddBlockLocatorHash(hash)
		if err != nil {
			return err
		}
	}

	p.QueueMessage(msg, nil)

	p.prevGetBlocksMtx.Lock()
	p.prevGetBlocksBegin = beginHash
	p.prevGetBlocksStop = stopHash
	p.prevGetBlocksMtx.Unlock()
	return nil
}

//pushgetheadersmsg sends a getblocks message for the provided block locator
//and stop hash.it will innore back-to-back duplicate requests
func (p *Peer) PushGetHeaderMsg(locator blockchain.BlockLocator, stopHash *chainhash.Hash) error {
	//extract the begin hash from the block locator ,if one was specified.
	//to use for filtering duplicate getheaders requests.
	var beginHash *chainhash.Hash
	if len(locator) > 0 {
		beginHash = locator[0]
	}

	//filter dupliacte getheaders requests.
	p.prevGetBlocksMtx.Lock()
	isDuplicate := p.prevGetHdrsStop != nil && p.prevGetHdrsBegin != nil &&
		beginHash != nil && stopHash.IsEqual(p.prevGetHdrsStop) && beginHash.IsEqual(p.prevGetHdrsBegin)

	if isDuplicate {
		log.Tracef("filtering duplicate [getheaders] with begin hash %v", beginHash)
		return nil
	}

	//construct the getheaders request and queue it to be sent.
	msg := wire.NewMsgGetHeaders()
	msg.HashStop = *stopHash
	for _, hash := range locator {
		err := msg.AddBlockLocatorHash(hash)
		if err != nil {
			return err
		}
	}

	p.QueueMessage(msg, nil)

	//update the previous getheaders request information for filterling duplicates.
	p.prevGetHdrsMtx.Lock()
	p.prevGetHdrsBegin = beginHash
	p.prevGetHdrsStop = stopHash
	p.prevGetHdrsMtx.Unlock()
	return nil

}

//pushrejectmsg sends a reject message for the provided command ,reject code.
//reject reason,and hash,the hash will only be used when the command is a tx
//or block and should be nil in other cases.the wait paremeter will cause the
//fucction to block until the reject message has actually been sent.
func (p *Peer) PushRejectMsg(command string, code wire.RejectCode, reason string, hash *chainhash.Hash, wait bool) {

	//do not bother sending the reject message if the protocol version is too low.
	if p.VersionKnown() && p.ProtocolVersion() < wire.RejectVersion {
		return
	}

	msg := wire.NewMsgReject(command, code, reason)
	if command == wire.CmdTx || command == wire.CmdBlock {
		if hash == nil {
			log.Warnf("sending a reject message for command "+
				"type %v which should have specified a hash"+
				"but does not", command)
			hash = &zeroHash
		}
		msg.Hash = *hash
	}

	//send the message without waiting if the caller has not requested
	if !wait {
		p.QueueMessage(msg, nil)
		return
	}

	//send the message and block until it has sent before returning
	doneChan := make(chan struct{}, 1)
	p.QueueMessage(msg, doneChan)
	<-doneChan

}

//handlepingmsg is invoked when p peer receives a ping bitcion message
//for recent clients (protocol version > bip0031version)it replies with
//a pong message,for older clients ,it does nothing and anything other
//than failure is considerd a successful ping
func (p *Peer) handlePingMsg(msg *wire.MsgPing) {
	//only reply with pong if the message is from a new enough client.
	if p.ProtocolVersion() > wire.BIP0031Version {
		// Include nonce from ping so pong can be identified.
		p.QueueMessage(wire.NewMsgPong(msg.Nonce), nil)
	}

}

// handlePongMsg is invoked when a peer receives a pong bitcoin message.  It
// updates the ping statistics as required for recent clients (protocol
// version > BIP0031Version).  There is no effect for older clients or when a
// ping was not previously sent.
func (p *Peer) handlePongMsg(msg *wire.MsgPong) {
	// Arguably we could use a buffered channel here sending data
	// in a fifo manner whenever we send a ping, or a list keeping track of
	// the times of each ping. For now we just make a best effort and
	// only record stats if it was for the last ping sent. Any preceding
	// and overlapping pings will be ignored. It is unlikely to occur
	// without large usage of the ping rpc call since we ping infrequently
	// enough that if they overlap we would have timed out the peer.
	if p.ProtocolVersion() > wire.BIP0031Version {
		p.statsMtx.Lock()
		if p.lastPingNonce != 0 && msg.Nonce == p.lastPingNonce {
			p.lastPingMicros = time.Since(p.lastPingTime).Nanoseconds()
			p.lastPingMicros /= 1000 // convert to usec.
			p.lastPingNonce = 0
		}
		p.statsMtx.Unlock()
	}
}

//readmessage reads next bitcoin message from the peer with logging

func (p *Peer) readMessage(encoding wire.MessageEncoding) (wire.Message, []byte, error) {
	n, msg, buf, err := wire.ReadMessageWithEncodingN(p.conn,
		p.ProtocolVersion(), p.cfg.ChainParams.Net, encoding)
	atomic.AddUint64(&p.bytesReceived, uint64(n))
	if p.cfg.Listeners.OnRead != nil {
		p.cfg.Listeners.OnRead(p, n, msg, err)
	}
	if err != nil {
		return nil, nil, err
	}

	// Use closures to log expensive operations so they are only run when
	// the logging level requires it.
	log.Debugf("%v", newLogClosure(func() string {
		// Debug summary of message.
		summary := messageSummary(msg)
		if len(summary) > 0 {
			summary = " (" + summary + ")"
		}
		return fmt.Sprintf("Received %v%s from %s",
			msg.Command(), summary, p)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(msg)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(buf)
	}))

	return msg, buf, nil
}

//writemessage sends a bitcoin message to the peer with logging
func (p *Peer) writeMessage(msg wire.Message, enc wire.MessageEncoding) error {
	//do not anything if we are disconnecting .
	if atomic.LoadInt32((&p.disconnect)) != 0 {
		return nil
	}

	//use closures to log expensive operations so they are only run when
	//the logging level requires it.
	log.Debugf("%v", newLogClosure(func() string {
		//debug summary of message
		summary := messageSummary(msg)
		if len(summary) > 0 {
			summary = "(" + summary + ")"
		}
		return fmt.Sprintf("sending %v%s to %s", msg.Command(), summary, p)
	}))

	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(msg)
	}))

	log.Tracef("%v", newLogClosure(func() string {
		var buf bytes.Buffer
		_, err := wire.WriteMessageWithEncodingN(&buf, msg, p.ProtocolVersion(), p.cfg.ChainParams.Net, enc)
		if err != nil {
			return err.Error()
		}
		return spew.Sdump(buf.Bytes())
	}))

	//write the message to the peer
	n, err := wire.WriteMessageWithEncodingN(p.conn, msg, p.ProtocolVersion(), p.cfg.ChainParams.Net, enc)
	atomic.AddUint64(&p.bytesSent, uint64(n))
	if p.cfg.Listeners.OnWrite != nil {
		p.cfg.Listeners.OnWrite(p, n, msg, err)
	}

	return err

}

//isallowedreadrerror returns whether or not passed error is allowed without
//disconnecting the peer.in particular. regression tests need to be allowed
//to send malformed message without the peer being disconnected.
func (p *Peer) isAllowedReadError(err error) bool {
	//only allow read errors in regression test mode.
	if p.cfg.ChainParams.Net != wire.TestNet {
		return false
	}

	//do not allow the error if it is not specifically a malformed message error.
	if _, ok := err.(*wire.MessageError); !ok {
		return false
	}

	//do not allow the error if it is not coming from localhost or the
	//hostname can not be determined for some reason.
	host, _, err := net.SplitHostPort(p.addr)
	if err != nil {
		return false
	}

	if host != "127.0.0.1" && host != "localhost" {
		return false
	}

	//allowed if all checks passed.
	return true

}

// shouldHandleReadError returns whether or not the passed error, which is
// expected to have come from reading from the remote peer in the inHandler,
// should be logged and responded to with a reject message.
func (p *Peer) shouldHandleReadError(err error) bool {
	// No logging or reject message when the peer is being forcibly
	// disconnected.
	if atomic.LoadInt32(&p.disconnect) != 0 {
		return false
	}

	// No logging or reject message when the remote peer has been
	// disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

//maybeadddeadline potentially adds a deadline for the appropriate expected
//response for the passed wire protocol command to the pending responses map
func (p *Peer) maybeAddDeadline(pendingResponses map[string]time.Time, msgCmd string) {
	//setup a dealine for each message being sent that expects a response.

	//note:pings are intentionally igmored here since they are typcicaly
	//sent asynchroonouly and as a result of a long backlock of message.
	//such as is typical in the case of initial block download. the respose
	//won be received in time.
	deadline := time.Now().Add(stallReponseTimeout)
	// Expects a verack message.
	pendingResponses[wire.CmdVerAck] = deadline
	switch msgCmd {

	case wire.CmdMemPool:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetBlocks:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetData:
		// Expects a block, merkleblock, tx, or notfound message.
		pendingResponses[wire.CmdBlock] = deadline
		pendingResponses[wire.CmdMerkleBlock] = deadline
		pendingResponses[wire.CmdTx] = deadline
		pendingResponses[wire.CmdNotFound] = deadline

	case wire.CmdGetHeaders:
		// Expects a headers message.  Use a longer deadline since it
		// can take a while for the remote peer to load all of the
		// headers.
		deadline = time.Now().Add(stallResponseTimeout * 3)
		pendingResponses[wire.CmdHeaders] = deadline
	}
}

//stallhandler handled stall detection for the peer .this entails keeping
//track of expected responses and assingning them deadline while accouting
//for the time spnet in callbacks ,it must be run as a goroutine
func (p *Peer) stallHandler() {
	//these variable are used to adjust the deadline times forward by the
	//time it takes callbacks to execute. this is done because new message
	//are not read until the previous one is finished processing which includes
	//callbacks .so the deadline for receiving a response for a given messge
	//must account for the processing time as well
	var handlerActive bool
	var handlersStartTime time.Time
	var deadlineOffset time.Duration

	//pendingresponses tracks the expected response deadline times
	pendingResponses := make(map[string]time.Time)

	//stallticker is used to periodically check pending responsed that
	//have exceded the expected deadline and disconnect the peer due to
	//stalling
	stallTicker := time.NewTimer(stallTrickTinterval)
	defer stallTicker.Stop()

	//isstopped is used to detect when both ipput and output handler
	//goroutine are done.
	var ioStopped bool
out:
	for {
		select {
		case msg := <-p.stallControl:
			switch msg.command {

				//add a deadline for the expected response
				//message if needed.
				p.maybeAddDeadline(pendingResponses, msg.message.Command())
			case sccReceiveMessage:
				//remove receied message from the ecpeceed response map.
				//since certain commands expect one of a group of response.
				//remove everything in the expected group accordingly.
				switch msgCmd := msg.message.Command(); msgCmd {
				case wire.CmdBlock:
					fallthrough
				case wire.CmdMerkleBlock:
					fallthrough
				case wire.CmdTx:
					fallthrough
				case wire.CmdNotFound:
					delete(pendingResponses, wire.CmdBlock)
					delete(pendingResponses, wire.CmdMerkleBlock)
					delete(pendingResponses, wire.CmdTx)
					delete(pendingResponses, wire.CmdNotFound)

				default:
					delete(pendingResponses, msgCmd)

				}

			case sccHandlerStart:
				//warn on unbalanced callback signaling
				if handlerActive {
					log.Warn("reveived handler start" +
						"control command while a " +
						"handler is already active")
					continue
				}

				handlerActive = true
				handlersStartTime = time.Now()

			case sccHandlerDone:
				//warn on unbalanced callback singnaling
				if !handlerActive {
					log.Warn("received handle done" +
						"control command when a " +
						"handler is not already active")
					continue
				}

				//extend active deadlines by the time it took
				//to execute the callback.
				duration := time.Since(handlersStartTime)
				deadlineOffset += duration
				handlerActive = false

			default:
				log.Warnf("unsupporeted message command %v", msg.command)
			}

		case <-stallTicker.C:
			//calculate the offset to apply to the deadline based on how long
			//the handlers have taken to execute since the last tick.
			now := time.Now()
			offset := deadlineOffset
			if handlerActive {
				offset += now.Sub(handlersStartTime)
			}

			//disconcet the peer if any of the pending responses
			//do not arrive their adjusted deadline.
			for command, deadline := range pendingResponses {
				if now.Before(deadline.Add(offset)) {
					continue
				}

				log.Debugf("peer %s appers to be stalled or "+
					"missbehaving,%s timeout -- "+"disconnecting", p, command)
				p.Disconnect
				break
			}

			//reset the dealine offset for the next tick
			deadlineOffset = 0

		case <-p.inQuit:
			//the stall handler can exit once both the input and output
			//handler goroutines are done.
			if ioStopped {
				break out
			}

			ioStopped = true

		}

	}

	//drwin any wait channels before going away so there is nothing left
	//waring on this goroutine.
cleanup:
	for {
		select {
		case <-p.stallControl
		default:
			break cleanup
		}
	}
	log.Tracef("peer stall handler done for %s", p)

}

//ishandler handles all incoming message for the peer .it must be run
//as a goroutine.
func (p *Peer) inHandler() {
	//the timer is stopped when a new message is received and reset after it
	//is processed.
	idleTimer := time.AfterFunc(idleTimeout, func() {
		log.Warnf("peer %s no answer for %s -- disconnecting ", p, idleTimeout)
		p.Disconnect()
	})

out:
	for atomic.LoadInt32(&p.disconnect) == 0 {
		//read a message and stop the idle timer as soon as the read
		//is done the timer is reset below for the next iteration if
		//needed.
		rmsg, buf, err := p.readMessage(p.wireEncoding)
		idleTimer.Stop()
		if err != nil {
			//in order to allow regerssion tests with malformed message
			//do not disconnect the peer when we are in regression test
			//and the error is one of the allowed errors.
			if p.isAllowedReadError(err) {
				log.Errorf("allowed test error from %s:%v", p, err)
				continue
			}

			//only log the error and send reject message if the local peer
			//is not forcibly disconnecting and the remote peer has not
			//disconnected.
			if p.shouldHandleReadError(err) {
				errMsg := fmt.Sprintf("can not read message from %s:%v", p, err)
				if err != io.ErrUnexpectedEOF {
					log.Errorf(errMsg)
				}
				//push a reject message for the malfromed message
				//and wait for the message to be sent before disconnceting

				p.PushRejectMsg("malformed ", wire.RejectMalformed, errMsg, nil, true)

			}

			break out

		}
		atomic.StoreInt64(&p.lastRecv, time.Now().Unix())
		p.stallControl <- stallControlMsg{sccReceiveMessage, rmsg}

		//handle each supported message type
		p.stallControl <- stallControlMsg{sccHandlerStart, rmsg}
		switch msg := rmsg.(type) {
		case *wire.MsgVersion:
			//limit to one version message per message.
			p.PushRejectMsg(msg.Command(), wire.RejectDuplicate,
				"duplicate version message", nil, true)
			break out

		case *wire.MsgVerAck:
			//limit to one verack message per peer.
			p.PushRejectMsg(
				msg.Command(), wire.RejectDuplicate,
				"duplicate verack message", nil, true)
			break out

		case *wire.MsgGetAddr:
			if p.cfg.Listeners.OnGetAddr != nil {
				p.cfg.Listeners.OnGetAddr(p, msg)
			}

		case *wire.MsgAddr:
			if p.cfg.Listeners.OnAddr != nil {
				p.cfg.Listeners.OnAddr(p, msg)
			}

		case *wire.MsgPing:
			p.handlePingMsg(msg)
			if p.cfg.Listeners.OnPing != nil {
				p.cfg.Listeners.OnPing(p, msg)
			}

		case *wire.MsgPong:
			p.handlePongMsg(msg)
			if p.cfg.Listeners.OnPong != nil {
				p.cfg.Listeners.OnPong(p, msg)
			}

		case *wire.MsgAlert:
			if p.cfg.Listeners.OnAlert != nil {
				p.cfg.Listeners.OnAlert(p, msg)
			}

		case *wire.MsgMemPool:
			if p.cfg.Listeners.OnMemPool != nil {
				p.cfg.Listeners.OnMemPool(p, msg)
			}

		case *wire.MsgTx:
			if p.cfg.Listeners.OnTx != nil {
				p.cfg.Listeners.OnTx(p, msg)
			}

		case *wire.MsgBlock:
			if p.cfg.Listeners.OnBlock != nil {
				p.cfg.Listeners.OnBlock(p, msg, buf)
			}

		case *wire.MsgInv:
			if p.cfg.Listeners.OnInv != nil {
				p.cfg.Listeners.OnInv(p, msg)
			}

		case *wire.MsgHeaders:
			if p.cfg.Listeners.OnHeaders != nil {
				p.cfg.Listeners.OnHeaders(p, msg)
			}

		case *wire.MsgNotFound:
			if p.cfg.Listeners.OnNotFound != nil {
				p.cfg.Listeners.OnNotFound(p, msg)
			}

		case *wire.MsgGetData:
			if p.cfg.Listeners.OnGetData != nil {
				p.cfg.Listeners.OnGetData(p, msg)
			}

		case *wire.MsgGetBlocks:
			if p.cfg.Listeners.OnGetBlocks != nil {
				p.cfg.Listeners.OnGetBlocks(p, msg)
			}

		case *wire.MsgGetHeaders:
			if p.cfg.Listeners.OnGetHeaders != nil {
				p.cfg.Listeners.OnGetHeaders(p, msg)
			}

		case *wire.MsgGetCFilters:
			if p.cfg.Listeners.OnGetCFilters != nil {
				p.cfg.Listeners.OnGetCFilters(p, msg)
			}

		case *wire.MsgGetCFHeaders:
			if p.cfg.Listeners.OnGetCFHeaders != nil {
				p.cfg.Listeners.OnGetCFHeaders(p, msg)
			}

		case *wire.MsgGetCFCheckpt:
			if p.cfg.Listeners.OnGetCFCheckpt != nil {
				p.cfg.Listeners.OnGetCFCheckpt(p, msg)
			}

		case *wire.MsgCFilter:
			if p.cfg.Listeners.OnCFilter != nil {
				p.cfg.Listeners.OnCFilter(p, msg)
			}

		case *wire.MsgCFHeaders:
			if p.cfg.Listeners.OnCFHeaders != nil {
				p.cfg.Listeners.OnCFHeaders(p, msg)
			}

		case *wire.MsgFeeFilter:
			if p.cfg.Listeners.OnFeeFilter != nil {
				p.cfg.Listeners.OnFeeFilter(p, msg)
			}

		case *wire.MsgFilterAdd:
			if p.cfg.Listeners.OnFilterAdd != nil {
				p.cfg.Listeners.OnFilterAdd(p, msg)
			}

		case *wire.MsgFilterClear:
			if p.cfg.Listeners.OnFilterClear != nil {
				p.cfg.Listeners.OnFilterClear(p, msg)
			}

		case *wire.MsgFilterLoad:
			if p.cfg.Listeners.OnFilterLoad != nil {
				p.cfg.Listeners.OnFilterLoad(p, msg)
			}

		case *wire.MsgMerkleBlock:
			if p.cfg.Listeners.OnMerkleBlock != nil {
				p.cfg.Listeners.OnMerkleBlock(p, msg)
			}

		case *wire.MsgReject:
			if p.cfg.Listeners.OnReject != nil {
				p.cfg.Listeners.OnReject(p, msg)
			}

		case *wire.MsgSendHeaders:
			p.flagsMtx.Lock()
			p.sendHeadersPreferred = true
			p.flagsMtx.Unlock()

			if p.cfg.Listeners.OnSendHeaders != nil {
				p.cfg.Listeners.OnSendHeaders(p, msg)
			}

		default:
			log.Debugf("Received unhandled message of type %v "+
				"from %v", rmsg.Command(), p)
		}
		p.stallControl <- stallControlMsg{sccHandlerDone, rmsg}

		//a message was reject so reset the idle timer
		idleTimer.Reset(idleTimeout)

	}
	//ensure the idle timer is stopped to avoid leaking the resourece
	idleTimer.Stop()

	//ensure concetion is closed.
	p.Disconnect()

	close(p.inQuit)
	log.Tracef("peer input handler done for %s ", p)

}

//queuehandler handles the queuing of outgoing data for the peer .this runas
//as a muxer for various of input so we can ensure that server and
//peer handlers will not block on us sending a message .that data is
//then paased on to outhandler to be actully written.
func (p *Peer) queueHandler() {
	pendingMsgs := list.New()
	invSendQueue := list.New()
	trickleTicker := time.NewTicker(p.cfg.TrickleInterval)
	defer trickleTicker.Stop()

	//we keep the waiting flag so that we know if we have a message
	//queued to hte outhander or not .we could use the presence of a
	//head of the list for this but we have rather racy concerns about
	//wherther it has gotten it at cleanup time - and thus who sendd on
	//the message. done channel .to avoid such confusion we keep a differrent
	//flag and pendingmsgs only contains messages that we have not yet passed to
	//outhandler.

	waiting := false

	//to avoid duplication below.

	queuePacket := func(msg outMsg, list *list.List, waiting bool) bool {
		if !waiting {
			p.sendQueue <- msg
		} else {
			list.PushBack(msg)
		}

		//we are always wating now.
		return true
	}

out:
	for {
		select {
		case msg := <-p.outputQueue:
			waitting = queuePacket(msg, pendingMsgs, waiting)

		//this channel is notify when a message has been sent across
		//the netword socket
		case <-p.sendDoneQueue:
			//no longer waiting if there are no more message
			//in the peding message queue.
			next := pendingMsgs.Front()
			if next == nil {
				waiting = false
				continue
			}

			//notify the outhanler about the next item to
			//asychronouly send.
			val := pendingMsgs.Remove(next)
			p.sendQueue <- val.(outMsg)

		case iv := <-p.outputInvChan:
			//no handshakes?they will find out soon enouth.
			if p.VersionKnown() {
				//if this is a new block,then we will blast it
				//out immediately .sipping the ivv trickle queue.
				if iv.Type == wire.InvTypeBlock || iv.Type == wire.InvTypeWitnessBlock {
					invMsg := wire.NewMsgInvSizeHint(1)
					invMsg.AddInvVect(iv)
					waiting = queuePacket(outMsg{msg: invMsg}, pendingMsgs, waiting)
				} else {
					invSendQueue.PushBack(iv)
				}

			}
		case <-trickleTicker.C:

			//do not seding anything if we are disconneting or there
			//is no quued inventory .
			//version is konwn if send queue has any entires.
			if atomic.LoadInt32(&p.disconnect) != 0 || invSendQueue.Len() == 0 {
				continue
			}

			//create and send as many inv message as needed to drain the inentroy
			//send queueu.

			invMsg := wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
			for e := invSendQueue.Front(); e != nil; e = invSendQueue.Front() {
				iv := invSendQueue.Remove(e).(*wire.InvVect)

				//dont send invetroy that became known after the initial check
				if p.knownInventory.Exists(iv) {
					continue
				}

				invMsg.AddInvVect(iv)
				if len(invMsg.InvList) >= maxInvTrickleSize {
					waiting = queuePacket(
						outMsg{msg: invMsg},
						pendingMsgs, waiting)
					invMsg = wire.NewMsgInvSizeHint(uint(invSendQueue.Len()))
				}

				//add the inventory that is being relayed to the known invenory for
				//for the peer.
				p.AddKnownInventory(iv)

			}

			if len(invMsg.InvList) > 0 {
				waiting = queuePacket(outMsg{msg: invMsg},
					pendingMsgs, waiting)
			}

		case <-p.quit:
			break out


		}

	}

	//drain wany wait channels before we go away so we do not leave
	//something waiting for us.
	for e := pendingMsgs.Front(); e != nil; e = pendingMsgs.Front() {
		val := pendingMsgs.Remove(e)
		msg := val.(outMsg)
		if msg.doneChan != nil {
			msg.doneChan <- struct{}{}
		}

	}
cleanup:

	for {
		select {
		case msg := <-p.outputQueue:
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
		case <-p.outputInvChan:
		//just drain channel
		default:
			break cleanup
		}
	}

	close(p.queueQuit)
	log.Tracef("peer queue hanler done for %s", p)

}




