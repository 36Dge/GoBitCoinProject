package rpcclient

import (
	"BtcoinProject/chaincfg"
	"bytes"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/websocket"
	"io"
	"io/ioutil"
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

// handleSendPostMessage handles performing the passed HTTP request, reading the
// result, unmarshalling it, and delivering the unmarshalled result to the
// provided response channel.
func (c *Client) handleSendPostMessage(details *sendPostDetails) {
	jReq := details.jsonRequest
	log.Tracef("Sending command [%s] with id %d", jReq.method, jReq.id)
	httpResponse, err := c.httpClient.Do(details.httpRequest)
	if err != nil {
		jReq.responseChan <- &response{err: err}
		return
	}

	// Read the raw bytes and close the response.
	respBytes, err := ioutil.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		err = fmt.Errorf("error reading json reply: %v", err)
		jReq.responseChan <- &response{err: err}
		return
	}

	// Try to unmarshal the response as a regular JSON-RPC response.
	var resp rawResponse
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		// When the response itself isn't a valid JSON-RPC response
		// return an error which includes the HTTP status code and raw
		// response bytes.
		err = fmt.Errorf("status code: %d, response: %q",
			httpResponse.StatusCode, string(respBytes))
		jReq.responseChan <- &response{err: err}
		return
	}

	res, err := resp.result()
	jReq.responseChan <- &response{result: res, err: err}
}

//sendposthanlder handler all outgoing messages when hte client is running
//in http POst mode .it uses a  buffered channel to serialize output message
// while allowing the sender to continue running asynchorounouly.it must be run
//as a gorouine.
func (c *Client) sendPostHandler() {
out:
	for {
		//send any messages ready for send until the shutdown channel
		//is closed
		select {
		case details := <-c.sendPostChan:
			c.handleSendPostMessage(details)


		case <-c.shutdown:
			break out
		}
	}

	//drain any wait channnels before exiting so nothing is left waiting
	//around to send.

cleanup:
	for {
		select {
		case details := <=c.sendPostChan:
			details.jsonRequest.responseChan <- &response{
				result: nil,
				err:    ErrClientShutdown,
			}
		default:
			break cleanup
		}
	}

	c.wg.Done()
	log.Tracef("RPC client send hanlder done for %s", c.config.Host)

}

//sendpostrequest sends the passed http request to the Rpc sever using the
//http client associated with the client it is backed by a buffered channnel
//so it will not block until the send channel is full.
func (c *Client) sendPostRequest(httpReq *http.Request, jReq *jsonRequest) {
	//dont send the message if shutting dwon.
	select {
	case <-c.shutdown:
		jReq.responseChan <- &reponse{result: nil, err: ErrClientShutdown}
	default:

	}
	c.sendPostChan <- &sendPostDetails{
		httpRequest: jReq,
		jsonRequest: httpReq,
	}
}

func newFutureError(err error) chan *reponse {
	responseChan := make(chan *reponse, 1)
	responseChan <- &response{err: err}
	return responseChan
}

// receiveFuture receives from the passed futureResult channel to extract a
// reply or any errors.  The examined errors include an error in the
// futureResult and the error in the reply from the server.  This will block
// until the result is available on the passed channel.
func receiveFuture(f chan *response) ([]byte, error) {
	// Wait for a response on the returned channel.
	r := <-f
	return r.result, r.err
}

//sendpost sends the passed request to the server by issuing an http post
//request using the provided response channel for the reply .typically a new
//connection is opened and closed for each command when using this method.
//however,the underlying http Client might coalesce multiple commnads
//depending on several factors incluing the remote server configuration.
func (c *Client) sendPost(jReq *jsonRequest) {
	//generate a request to configed RPC sever.
	protocol := "http"
	if !c.config.DisableTLS {
		protocol = "https"
	}
	url := protocol + "://" + c.config.Host
	bodyReader := bytes.NewReader(jReq.marshalledJSON)
	httpReq, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		jReq.responseChan <- &response{result: nil, err: err}
		return
	}

	httpReq.Close = true
	httpReq.Header.Set("Content-type", "applcation/json")

	//Configure basic access authorization.
	httpReq.SetBasicAuth(c.config.User, c.config.Pass)

	log.Tracef("sending command[%s]with id %d", jReq.method, jReq.id)
	c.sendPostRequest(httpReq, jReq)
}
//sendrequest sends the passed json request to the associated server using
//the provided respose channel for the reply.it handles both websocket and thup
//post mode depending on the configuration of the client.
func (c *Client) sendRequest(jReq *jsonRequest){
	//choose which marshal and send function to use depending on whether
	//the client running in http post mode or not.when runnning in http
	//post mode .the command is issued via an http client otherwise .
	//the command is issued via the asynchornous websoeck channel.
	if c.config.HTTPPostMode {
		c.sendPost(jReq)
		return
	}

	//check whehter the websocket connection has never been established.
	//in which case the handler goroutines are not running
	select {
	case <-c.connEstablished:
	default:
		jReq.responseChan <- &response{err : ErrClientNotConnected}
		return
	}

	//add the request to the internal tracking map so the response from
	//the remote server can be porperly detected and routed to the response
	//channel then send the marshanlled request via the websocket
	//connetion.
	if err := c.addRequest(jReq);err != nil {
		jReq.responseChan <- &response{err:err}
		return
	}

	log.Tracef("Sending command [%s] with id %d",jReq.method,jReq.id)
	c.sendMessage(jReq.marshalledJSON)

}






// sendCmdAndWait sends the passed command to the associated server, waits
// for the reply, and returns the result from it.  It will return the error
// field in the reply if there is one.
func (c *Client) sendCmdAndWait(cmd interface{}) (interface{}, error) {
	// Marshal the command to JSON-RPC, send it to the connected server, and
	// wait for a response on the returned channel.
	return receiveFuture(c.sendCmd(cmd))
}

// Disconnected returns whether or not the server is disconnected.  If a
// websocket client was created but never connected, this also returns false.
func (c *Client) Disconnected() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	select {
	case <-c.connEstablished:
		return c.disconnected
	default:
		return false
	}
}

















// WaitForShutdown blocks until the client goroutines are stopped and the
// connection is closed.
func (c *Client) WaitForShutdown() {
	c.wg.Wait()
}

//connconfig describes the connection configuration parameters for the client
//this
type ConnConfig struct {
	// Host is the IP address and port of the RPC server you want to connect
	// to.
	Host string

	// Endpoint is the websocket endpoint on the RPC server.  This is
	// typically "ws".
	Endpoint string

	// User is the username to use to authenticate to the RPC server.
	User string

	// Pass is the passphrase to use to authenticate to the RPC server.
	Pass string

	// Params is the string representing the network that the server
	// is running. If there is no parameter set in the config, then
	// mainnet will be used by default.
	Params string

	// DisableTLS specifies whether transport layer security should be
	// disabled.  It is recommended to always use TLS if the RPC server
	// supports it as otherwise your username and password is sent across
	// the wire in cleartext.
	DisableTLS bool

	// Certificates are the bytes for a PEM-encoded certificate chain used
	// for the TLS connection.  It has no effect if the DisableTLS parameter
	// is true.
	Certificates []byte

	// Proxy specifies to connect through a SOCKS 5 proxy server.  It may
	// be an empty string if a proxy is not required.
	Proxy string

	// ProxyUser is an optional username to use for the proxy server if it
	// requires authentication.  It has no effect if the Proxy parameter
	// is not set.
	ProxyUser string

	// ProxyPass is an optional password to use for the proxy server if it
	// requires authentication.  It has no effect if the Proxy parameter
	// is not set.
	ProxyPass string

	// DisableAutoReconnect specifies the client should not automatically
	// try to reconnect to the server when it has been disconnected.
	DisableAutoReconnect bool

	// DisableConnectOnNew specifies that a websocket client connection
	// should not be tried when creating the client with New.  Instead, the
	// client is created and returned unconnected, and Connect must be
	// called manually.
	DisableConnectOnNew bool

	// HTTPPostMode instructs the client to run using multiple independent
	// connections issuing HTTP POST requests instead of using the default
	// of websockets.  Websockets are generally preferred as some of the
	// features of the client such notifications only work with websockets,
	// however, not all servers support the websocket extensions, so this
	// flag can be set to true to use basic HTTP POST requests instead.
	HTTPPostMode bool

	// EnableBCInfoHacks is an option provided to enable compatibility hacks
	// when connecting to blockchain.info RPC server
	EnableBCInfoHacks bool
}

// wsReconnectHandler listens for client disconnects and automatically tries
// to reconnect with retry interval that scales based on the number of retries.
// It also resends any commands that had not completed when the client
// disconnected so the disconnect/reconnect process is largely transparent to
// the caller.  This function is not run when the DisableAutoReconnect config
// options is set.
//
// This function must be run as a goroutine.

func (c *Client) wsReconnectHandler() {
out:
	for {
		select {
		case <-c.disconnect:
		//on disconnect,fallthrough to reestalish the connection.
		case <-c.shutdown:
			break out

		}
	}
}
