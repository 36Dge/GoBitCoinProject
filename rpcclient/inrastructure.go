package rpcclient

import (
	"BtcoinProject/chaincfg"
	"container/list"
	"encoding/json"
	"errors"
	"github.com/btcsuite/websocket"
	"io"
	"math"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//errIvvalAuth is an error to describe the condition where the client
	//is either unable to authenticate or the specified endpoint is incorrrett.
	ErrInvalidAuth = errors.New("authentication failure")

	//errinvalidendpoint is an error to describe the condition where the
	//websocket handshake failed with specified endpoint
	ErrInvalidEndpoint = errors.New("the endpoint either does not support " + "websocket or does not exist")

	//errClientNotConnected  is an error to descaribe the condition where websocket
	//clente has been created ,but the connection was never established .this condition
	//differs form errclientdisconnect ,which represents an estanlished connection that
	//was lost.
	ErrClientNotConnected = errors.New("the client was never connected")

	ErrClientDisconnect = errors.New("the client has been disconnected")

	ErrClientShutdown = errors.New("the client has been shutdown")

	ErrNotWebsocketClient = errors.New("client is not configured for " + "websockers")

	ErrClientAlreadyConnected = errors.New("websocket client has already" + "connected")
)

const (
	//sendbuffersize is the number of elements the websocket send channnel
	//can queue before blocking .
	sendBufferSize = 50

	//sendPostbuffersize is the number of elements the https post send
	//channel can queue before blocking
	sendPostBufferSize = 100

	//connectionRetryInterva is the amount of time to wait in between
	//retries when automationcally reconnectiong to an RPc server.
	connectionRetryInterval = time.Second * 5
)

//sendpostdetails hourse an http post request to send to an rpc server
//as well as the original json-rpc command and a channel to reply on
//when the server responds with the result.
type sendPostDetails struct {
	httpRequest *http.Request
	jsonRequest *jsonRequest
}

//jsonrequest holds information about a json request that is used to
//properly detect.interpret.and deliver a relay to it.
type jsonRequest struct {
	id             uint64
	method         string
	cmd            interface{}
	marshalledJSON []byte
	responseChan   chan *response
}

//backend version representst the version of the backend the client is current
//connetcted to
type BackendVersion uint8

const (
	// BitcoindPre19 represents a bitcoind version before 0.19.0.
	BitcoindPre19 BackendVersion = iota

	// BitcoindPost19 represents a bitcoind version equal to or greater than
	// 0.19.0.
	BitcoindPost19

	// Btcd represents a catch-all btcd version.
	Btcd
)

//client represents a bitclient Rpc client which allows easy access to the
//various rpc methods available on a bitcoin rpc server .each of the wrapper
//fucntions handle the details of converting the passed and return types to and
//from the underlying json types which are required for the json-rpc invocations.

// The client provides each RPC in both synchronous (blocking) and asynchronous
// (non-blocking) forms.  The asynchronous forms are based on the concept of
// futures where they return an instance of a type that promises to deliver the
// result of the invocation at some future time.  Invoking the Receive method on
// the returned future will block until the result is available if it's not
// already.

type Client struct {
	id uint64 // atomic, so must stay 64-bit aligned

	// config holds the connection configuration assoiated with this client.
	config *ConnConfig

	// chainParams holds the params for the chain that this client is using,
	// and is used for many wallet methods.
	chainParams *chaincfg.Params

	// wsConn is the underlying websocket connection when not in HTTP POST
	// mode.
	wsConn *websocket.Conn

	// httpClient is the underlying HTTP client to use when running in HTTP
	// POST mode.
	httpClient *http.Client

	// backendVersion is the version of the backend the client is currently
	// connected to. This should be retrieved through GetVersion.
	backendVersionMu sync.Mutex
	backendVersion   *BackendVersion

	// mtx is a mutex to protect access to connection related fields.
	mtx sync.Mutex

	// disconnected indicated whether or not the server is disconnected.
	disconnected bool

	// retryCount holds the number of times the client has tried to
	// reconnect to the RPC server.
	retryCount int64

	// Track command and their response channels by ID.
	requestLock sync.Mutex
	requestMap  map[uint64]*list.Element
	requestList *list.List

	// Notifications.
	ntfnHandlers  *NotificationHandlers
	ntfnStateLock sync.Mutex
	ntfnState     *notificationState

	// Networking infrastructure.
	sendChan        chan []byte
	sendPostChan    chan *sendPostDetails
	connEstablished chan struct{}
	disconnect      chan struct{}
	shutdown        chan struct{}
	wg              sync.WaitGroup
}

// NextID returns the next id to be used when sending a JSON-RPC message.  This
// ID allows responses to be associated with particular requests per the
// JSON-RPC specification.  Typically the consumer of the client does not need
// to call this function, however, if a custom request is being created and used
// this function should be used to ensure the ID is unique amongst all requests
// being made.

func (c *Client) NextID() uint64 {
	return atomic.AddUint64(&c.id, 1)
}

//addrequest associates the passed jsonrequest with its id ,
//this allows the response form the remote server to be unmarshalled
//to the appropriate type and sent to the specified channel when it
//received .
//if the client has already begun shutting down,errclientshutdown is
//is returned and the request is not added.
//this function is safe for concurrent access.
func (c *Client) addRequest(jReq *jsonRequest) error {
	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	select {
	case <-c.shutdown:
		return ErrClientShutdown
	default:

	}

	element := c.requestList.PushBack(jReq)
	c.requestMap[jReq.id] = element
	return nil

}

//dremoverequest returns and remove the jsonreuqest which contains the resooponse
//channel and origin methos associated with the passed id or nil if there is
//no association .
//this function is  safe for concurrent access.
func (c *Client) removeRequest(id uint64) *jsonRequest {
	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	element := c.requestMap[id]
	if element != nil {
		delete(c.requestMap, id)
		request := c.requestList.Remove(element).(*jsonRequest)
		return request
	}
	return nil
}

// removeAllRequests removes all the jsonRequests which contain the response
// channels for outstanding requests.
//
// This function MUST be called with the request lock held.
func (c *Client) removeAllRequests() {
	c.requestMap = make(map[uint64]*list.Element)
	c.requestList.Init()
}

//trackregiseredntfs examines the passed command to see if it is one of
//the notificaiion commands and updated the notification state that is used
//to automatically re_establish regisered notifications on reconnects.
func (c *Client) trackRegisteredNtfns(cmd interface{}) {
	//nothing to do if the caller is not interested in notifications
	if c.ntfnHandlers == nil {
		return
	}

	c.ntfnStateLock.Lock()
	defer c.ntfnStateLock.Unlock()

	switch bcmd := cmd.(type) {
	case *btcjson.NotifyBlocksCmd:
		c.ntfnState.notifyBlocks = true

	case *btcjson.NotifyNewTrasnactionsCmd:
		if bcmd.Verbose != nil && *bcmd.Verbose {
			c.ntfnState.notifyNewTxVerbose = true
		} else {
			c.ntfnState.notifyNewTx = true

		}

	case *btcjson.NotifySpentCmd:
		for _, op := range bcmd.OutPoints {
			c.ntfnState.notifySpent[op] = struct{}{}
		}

	case *btcjson.NotifyReceivedCmd:
		for _, addr := range bcmd.Addresses {
			c.ntfnState.notifyReceived[addr] = struct{}{}
		}

	}

}

//inmessage is the fist type that an incoming message is unmarshalled
//into ,it suport both requests(for notification support) and response.
//the partially-unmarshalled message is a notification if the embeded id
//from the response is nil ,otherwise.it is a response.
type inMessage struct {
	ID *float64 `json:"id"`
	*rawNotification
	*rawResponse
}

//rawnotification is a partially-ummarshaled json-rpc notificaton
type rawNotification struct {
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

//rawresponse is  a partially-unmarshaled json -PRC resonse for this
//to be valid (according to json-rpc 1.0 spec) id may not be nil
type rawResponse struct {
	Result json.RawMessage   `json:"result"`
	Error  *btcjson.RPCError `json:"error"`
}

//response is the raw bytes of a json-rpc result,or the error if the response
//error object was non-null
type reponse struct {
	result []byte
	err    error
}

//result checks whether the ummarshaled response contains a non-nil error.
//returing an ummarshaled btcjson.rpcerror (or an unmarhsling error)if so.
//if the response is not an error the raw bytes of the request are returned
//for further unmashing into specific result types.
func (r rawResponse) result() (result []byte, err error) {
	if r.Error != nil {
		return nil, r.Error

	}

	return r.Result, err
}

//handlemessage is the main hanller for incoming notification and responses.
func (c *Client) handleMessage(msg []byte) {
	//attempt to unmarshal the message as either a notification or resplnse.
	var in inMessage
	in.rawResponse = new(rawResponse)
	in.rawNotification = new(rawNotification)
	err := json.Unmarshal(msg, &in)
	if err != nil {
		log.Warnf("remote server sent invalid message:%v", err)
		return
	}

	//json-rpc 1.0 notifications are requests with a unll id.

	if in.ID == nil {
		ntfn := in.rawNotification
		if ntfn == nil {
			log.Warn("malformed notification:missing" +
				"method and parameters")
			return
		}
		if ntfn.Method == "" {
			log.Warn("malformed notification :missing method")
			return
		}

		//params are not optional :nil isn,t valid (but len == 0is)
		if ntfn.Params == nil {
			log.Warn("malformed notification :missing params")
			return
		}

		//deliver the notification
		log.Tracef("receiver notificaton[%s]", in.Method)
		c.handleNotification(in.rawNotification)
		return
	}

	//ensure that in.id can be converted to an integer without loss of precisiosn
	if *in.ID < 0 || *in.ID != math.Trunc(*in.ID) {
		log.Warn("malformed response :invalid identifier")
		return
	}

	if in.rawResponse == nil {
		log.Warn("malformed response :missing result and error")
		return
	}

	id := uint64(*in.ID)
	log.Tracef("receiver response for id %d(result%s)", id, in.Result)

	//nothing more to do if there is no request asssociated with this reply
	if request == nil || request.responseChan == nil {
		log.Warnf("receiver unexpected reply :%s(id %d)", in.Result, id)
		return
	}

	//since the command was successful ,exaimine it to see if it is a
	//notification and if it is ,add it to the notification state so it
	//can automatically be re-established on reconnet.
	c.trackRegisteredNtfns(request.cmd)

	//deliver the response
	result, err := in.rawResponse.result()
	request.responseChan <- &response{result: result, err: err}

}

// shouldLogReadError returns whether or not the passed error, which is expected
// to have come from reading from the websocket connection in wsInHandler,
// should be logged.
func (c *Client) shouldLogReadError(err error) bool {
	// No logging when the connetion is being forcibly disconnected.
	select {
	case <-c.shutdown:
		return false
	default:
	}

	// No logging when the connection has been disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

//shouldlogreaderror returns whether or not the passed error ,wehich is
//to have come form reading from the websocket conncetion in wsinhanlder
//should be logged
func (c *Client) shouldLogReadError(err error) bool {
	//no logging when the connection is being forcibly disconnected
	select {
	case <-c.shutdown:
		return false
	default:

	}

	//no loggging when the connection has been disconneted.
	if err == io.EOF {
		return false

	}

	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false

	}
	return true
}

//wsinhanlder handles all incoming messages for the websocket connection
//associated with the client .it must be run as goroutinue.
func (c *Client) wsInHandler() {
out:
	for {
		//break out of the loop once the shutdown channel has been
		//closed .use a non-blocking select here so we fall through
		//otherwise.
		select {
		case <-c.shutdown
			break out
		default:

		}

		_, msg, err := c.wsConn.ReadMessage()
		if err != nil {
			//log the error if it is not due to disconnectiong .
			if c.shouldLogReadError(err) {
				log.Errorf("websocket receiver error from "+
					"%s :%v", c.config.Host, err)
			}
			break out

		}
		c.handleMessage(msg)
	}
	//ensure the connection is closed.
	c.Disconnect
	c.wg.Done()
	log.Tracef("RPC client input hanlder done for %s", c.config.Host)
}

// disconnectChan returns a copy of the current disconnect channel.  The channel
// is read protected by the client mutex, and is safe to call while the channel
// is being reassigned during a reconnect.
func (c *Client) disconnectChan() <-chan struct{} {
	c.mtx.Lock()
	ch := c.disconnect
	c.mtx.Unlock()
	return ch
}

// wsOutHandler handles all outgoing messages for the websocket connection.  It
// uses a buffered channel to serialize output messages while allowing the
// sender to continue running asynchronously.  It must be run as a goroutine.
func (c *Client) wsOutHandler() {
out:
	for {
		//send any message ready for send until the client is
		//disconneted closed.
		select {
		case msg := <-c.sendChan:
			err := c.wsConn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				c.Disconnect()
				break out

			}
		case <-c.disconnectChan():
			break out

		}
	}

	//drain any channels before existing so nothing is left waiting around
	//to send .
cleanup:
	for {
		select {
		case <-c.sendChan:
		default:
			break cleanup
		}
	}

	c.wg.Done()
	log.Tracef("RPC client output handler done for %s", c.config.Host)

}

//sendmessage sends the passed json to the connected server using the websocket
//connection ,it is backed by a buffered channnel ,so it will not block until
//the send channel is full.
func (c *Client) sendMessage(marshalledJSON []byte) {
	//don,t send the message if disconnectd.
	select {
	case c.sendChan <- marshalledJSON:
	case <-c.disconnectChan():
		return
	}
}

//reregisterNfts creates and sends commands needed to re_establish the crrent
//notification state associated with the client .it should only be called on
//reconnect by the resendRequests function.
func (c *Client) reregisterNtfns() error {
	//nothing to do if the caller if not interested in notifications.
	if c.ntfnHandlers == nil {
		return nil
	}

	//in order to avoid holding the lock on the notification state for the
	//entire time of the potentially long running RPCS issued below. make a
	//copy of it and work from that .

	//also.other commands will be running concurrently which could mofity
	//the notification state (while not under the lock of course ) which
	//also register it which the remote RPC server,so this prevents double
	//registerations.
	c.ntfnStateLock.Lock()
	stateCopy := c.ntfnState.Copy()
	c.ntfnStateLock.Unlock()

	//reregisster notifyblocks if needed.
	if stateCopy.notifyBlocks {
		log.Debugf("reregistering [notifyblcoks]")
		if err := c.notifyBlocks(); err != nil {
			return err
		}
	}

	//reregister notifynewtransaction if needed .
	if stateCopy.notifyNewTx || stateCopy.notifyNewTxVerbose {
		log.Debugf("reregistering [notifynewtrnasactions](verbose+%v)",
			stateCopy.notifyNewTxVerbose)
		err := c.notifyNewTransactions(stateCopy.notifyNewTxVerbose)
		if err != nil {
			return err
		}
	}

	//reregister the comfination of all pdreviouly registered nofityspent.
	//outpoints in one command if needed.
	nslen := len(stateCopy.notifySpent)
	if nslen > 0 {
		outpoints := make([]btcjson.OutPoint, 0, nslen)
		for op := range stateCopy.notifySpent {
			outpoints = append(outpoints, op)
		}

		log.Debugf("Reregistering [notifyspent] outpoints: %v", outpoints)
		if err := c.notifySpentInternal(outpoints).Receive(); err != nil {
			return err
		}
	}
	// Reregister the combination of all previously registered
	// notifyreceived addresses in one command if needed.
	nrlen := len(stateCopy.notifyReceived)
	if nrlen > 0 {
		addresses := make([]string, 0, nrlen)
		for addr := range stateCopy.notifyReceived {
			addresses = append(addresses, addr)
		}
		log.Debugf("Reregistering [notifyreceived] addresses: %v", addresses)
		if err := c.notifyReceivedInternal(addresses).Receive(); err != nil {
			return err
		}
	}

	return nil

}

//ignoreresends is a set of all methods for requests that are ""long running"
//are not be reissued by the client on reconnct.
var ignoreReseds = map[string]struct{}{
	"rescan": {},
}

//resendrequests resend any requests that had not completed when the client
//disconneted .it is intended to be called once the client has reconnected as
//a seperate goroutinue.
func (c *Client) resendRequests() {
	//set the notification state back up ,if anything goes worong
	//disconnect the client.
	if err := c.reregisterNtfns(); err != nil {
		log.Warnf("unable to re-establish notification state :%v", err)
		c.Disconnect()
		return

	}

	// Since it's possible to block on send and more requests might be
	// added by the caller while resending, make a copy of all of the
	// requests that need to be resent now and work from the copy.  This
	// also allows the lock to be released quickly.

	c.requestLock.Lock()
	resendReqs := make([]*jsonRequest, 0, c.requestList.Len())
	var nextElem *list.Element
	for e := c.requestList.Front(); e != nil; e = nextElem {
		nextElem = e.Next()

		jReq := e.Value.(*jsonRequest)
		if _, ok := ignoreReseds[jReq.method]; ok {
			//if a request is not on reconnet ,remove it form the request structure s ,since no repley
			//is expected
			delete(c.requestMap, jReq.id)
			c.requestList.Remove(e)
		} else {
			resendReqs = append(resendReqs, jReq)
		}
	}
	c.requestLock.Unlock()

	for _, jReq := range resendReqs {

		//stop resending cmmands if the client disconnected again
		//since the next reconnect will handle them.
		if c.Disconnected() {
			return
		}

		log.Tracef("sneding command[%s] with id %d", jReq.method,
			jReq.id)
		c.sendMessage(jReq.marshalledJSON)
	}

}
