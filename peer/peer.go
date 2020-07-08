package peer

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"net"
	"sync"
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

// PushGetBlocksMsg sends a getblocks message for the provided block locator
// and stop hash.  It will ignore back-to-back duplicate requests.
//
// This function is safe for concurrent access.
func (p *Peer) PushGetBlocksMsg(locator blockchain.BlockLocator, stopHash *chainhash.Hash) error {
	// Extract the begin hash from the block locator, if one was specified,
	// to use for filtering duplicate getblocks requests.
	var beginHash *chainhash.Hash
	if len(locator) > 0 {
		beginHash = locator[0]
	}

	// Filter duplicate getblocks requests.
	p.prevGetBlocksMtx.Lock()
	isDuplicate := p.prevGetBlocksStop != nil && p.prevGetBlocksBegin != nil &&
		beginHash != nil && stopHash.IsEqual(p.prevGetBlocksStop) &&
		beginHash.IsEqual(p.prevGetBlocksBegin)
	p.prevGetBlocksMtx.Unlock()

	if isDuplicate {
		log.Tracef("Filtering duplicate [getblocks] with begin "+
			"hash %v, stop hash %v", beginHash, stopHash)
		return nil
	}

	// Construct the getblocks request and queue it to be sent.
	msg := wire.NewMsgGetBlocks(stopHash)
	for _, hash := range locator {
		err := msg.AddBlockLocatorHash(hash)
		if err != nil {
			return err
		}
	}
	p.QueueMessage(msg, nil)

	// Update the previous getblocks request information for filtering
	// duplicates.
	p.prevGetBlocksMtx.Lock()
	p.prevGetBlocksBegin = beginHash
	p.prevGetBlocksStop = stopHash
	p.prevGetBlocksMtx.Unlock()
	return nil
}
