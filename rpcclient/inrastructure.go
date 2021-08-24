package rpcclient

import (
	"BtcoinProject/btcjson"
	"BtcoinProject/chaincfg"
	"bytes"
	"container/list"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/go-socks/socks"
	"github.com/btcsuite/websocket"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
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
		case details := <-c.sendPostChan:
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
		jReq.responseChan <- &response{result: nil, err: ErrClientShutdown}
	default:

	}
	c.sendPostChan <- &sendPostDetails{
		httpRequest: jReq,
		jsonRequest: httpReq,
	}
}

func newFutureError(err error) chan *response {
	responseChan := make(chan *response, 1)
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
func (c *Client) sendRequest(jReq *jsonRequest) {
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
		jReq.responseChan <- &response{err: ErrClientNotConnected}
		return
	}

	//add the request to the internal tracking map so the response from
	//the remote server can be porperly detected and routed to the response
	//channel then send the marshanlled request via the websocket
	//connetion.
	if err := c.addRequest(jReq); err != nil {
		jReq.responseChan <- &response{err: err}
		return
	}

	log.Tracef("Sending command [%s] with id %d", jReq.method, jReq.id)
	c.sendMessage(jReq.marshalledJSON)

}

// response is the raw bytes of a JSON-RPC result, or the error if the response
// error object was non-null.
type response struct {
	result []byte
	err    error
}

//sendCmd sends the passed command to the associated server and retuns a
//response channel on which the reply will be delivered at some point in
//in the future .it handles both websocket and http post mode depending on
//the configuation of the client.
func (c *Client) sendCmd(cmd interface{}) chan *response {
	//get the method associated with the command
	method, err := btcjson.CmdMethod(cmd)
	if err != nil {
		return newFutureError(err)
	}

	//marshal the command
	id := c.NextID()
	marshalledJSON, err := btcjson.MarshalCmd(id, cmd)
	if err != nil {
		return newFutureError(err)
	}

	//generate the request and send it along with a channel to respond on.
	responseChan := make(chan *response, 1)
	jReq := &jsonRequest{
		id:             id,
		method:         method,
		cmd:            cmd,
		marshalledJSON: marshalledJSON,
		responseChan:   responseChan,
	}
	c.sendRequest(jReq)

	return responseChan

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

//dodisconnect disconnercts the websocket associated with client if it
//has not already been disconnected .it will return fasle if the diconnect is
//not needed or client is running in http post mode.

//this function is safe for concurrent access.
func (c *Client) doDisconnect() bool {
	if c.config.EnableBCInfoHacks {
		return false
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	//nothing to do if already disconnectd .
	if c.disconnected {
		return false
	}

	log.Tracef("disconnecting RPC client %s", c.config.Host)
	close(c.disconnect)
	if c.wsConn != nil {
		c.wsConn.Close()

	}

	c.disconnected = true
	return true
}

//doshutdown closes theshutdown channel adn logs theshutdown unless shutdown
//is already in progeress in will returs false if the shutdown is not needed
//this function is safe for concurretn access.
func (c *Client) doShutdown() bool {
	//ignore the shutdown request if the client is already in the process
	//of shutting down or alredy shutdown.
	select {
	case <-c.shutdown:
		return false
	default:

	}

	log.Tracef("shutting down RPC client %s", c.config.Host)
	close(c.shutdown)
	return true
}

// Disconnect disconnects the current websocket associated with the client.  The
// connection will automatically be re-established unless the client was
// created with the DisableAutoReconnect flag.
//
// This function has no effect when the client is running in HTTP POST mode.
func (c *Client) Disconnect() {
	// Nothing to do if already disconnected or running in HTTP POST mode.
	if !c.doDisconnect() {
		return
	}

	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	// When operating without auto reconnect, send errors to any pending
	// requests and shutdown the client.
	if c.config.DisableAutoReconnect {
		for e := c.requestList.Front(); e != nil; e = e.Next() {
			req := e.Value.(*jsonRequest)
			req.responseChan <- &response{
				result: nil,
				err:    ErrClientDisconnect,
			}
		}
		c.removeAllRequests()
		c.doShutdown()
	}
}

// Shutdown shuts down the client by disconnecting any connections associated
// with the client and, when automatic reconnect is enabled, preventing future
// attempts to reconnect.  It also stops all goroutines.
func (c *Client) Shutdown() {
	// Do the shutdown under the request lock to prevent clients from
	// adding new requests while the client shutdown process is initiated.
	c.requestLock.Lock()
	defer c.requestLock.Unlock()

	// Ignore the shutdown request if the client is already in the process
	// of shutting down or already shutdown.
	if !c.doShutdown() {
		return
	}

	// Send the ErrClientShutdown error to any pending requests.
	for e := c.requestList.Front(); e != nil; e = e.Next() {
		req := e.Value.(*jsonRequest)
		req.responseChan <- &response{
			result: nil,
			err:    ErrClientShutdown,
		}
	}
	c.removeAllRequests()

	// Disconnect the client if needed.
	c.doDisconnect()
}

//start begins processing input and output message.
func (c *Client) start() {
	log.Tracef("starting RPC client %s", c.config.Host)

	//start the I/O porcesing handlers depending on whether the client is
	//in http post mode or thedefualt websocket mode.
	if c.config.HTTPPostMode {
		c.wg.Add(1)
		go c.sendPostHandler()
	} else {
		c.wg.Add(3)
		go func() {
			if c.ntfnHandlers != nil {
				if c.ntfnHandlers.OnClientConnected != nil {
					c.ntfnHandlers.OnClientConnected()
				}

			}
			c.wg.Done()
		}()
	}
	go c.wsInHandler()
	go c.wsOutHandler()
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

//newhttpclient returns a new http client that is configured according to hte
//proxy and TLS setting in the associated connection configuration.
func newHTTPClient(config *ConnConfig) (*http.Client, error) {
	//set porxy function if there is a porxy configured.
	var proxyFunc func(*http.Request) (*url.URL, error)
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, err
		}
		proxyFunc = http.ProxyURL(proxyURL)
	}

	//cofigure tls if needed
	var tlsConfig *tls.Config
	if !config.DisableTLS {
		if len(config.Certificates) > 0 {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(config.Certificates)
			tlsConfig = &tls.Config{RootCAs: pool}
		}
	}

	client := http.Client{
		Transport: &http.Transport{
			Proxy:           proxyFunc,
			TLSClientConfig: tlsConfig,
		},
	}
	return &client, nil
}

//dial opens a websocket connection using the passed connection confiuation
//details.
func dial(config *ConnConfig) (*websocket.Conn, error) {
	//setup TLS if not disabled.
	var tlsConfig *tls.Config
	var scheme = "ws"
	if !config.DisableTLS {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if len(config.Certificates) > 0 {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(config.Certificates)
			tlsConfig.RootCAs = pool
		}
		scheme = "wss"
	}

	//create a websocket dialer that will be used to make the connection
	//it is modified by the proxy setting below as needed.
	dialer := websocket.Dialer{TLSClientConfig: tlsConfig}

	//setup the proxy if one is configured.
	if config.Proxy != "" {
		proxy := &socks.Proxy{
			Addr:     config.Proxy,
			Username: config.ProxyUser,
			Password: config.ProxyPass,
		}
		dialer.NetDial = proxy.Dial
	}

	//the RPC server requires basic authorization ,so create a custom
	//request header with the authorization header set.
	login := config.User + ":" + config.Pass
	auth := "Basic" + base64.StdEncoding.EncodeToString([]byte(login))
	requestHeader := make(http.Header)
	requestHeader.Add("Authorization", auth)

	// Dial the connection.
	url := fmt.Sprintf("%s://%s/%s", scheme, config.Host, config.Endpoint)
	wsConn, resp, err := dialer.Dial(url, requestHeader)
	if err != nil {
		if err != websocket.ErrBadHandshake || resp == nil {
			return nil, err
		}

		// Detect HTTP authentication error status codes.
		if resp.StatusCode == http.StatusUnauthorized ||
			resp.StatusCode == http.StatusForbidden {
			return nil, ErrInvalidAuth
		}

		// The connection was authenticated and the status response was
		// ok, but the websocket handshake still failed, so the endpoint
		// is invalid in some way.
		if resp.StatusCode == http.StatusOK {
			return nil, ErrInvalidEndpoint
		}

		// Return the status text from the server if none of the special
		// cases above apply.
		return nil, errors.New(resp.Status)
	}
	return wsConn, nil

}

const (
	// bitcoind19Str is the string representation of bitcoind v0.19.0.
	bitcoind19Str = "0.19.0"

	// bitcoindVersionPrefix specifies the prefix included in every bitcoind
	// version exposed through GetNetworkInfo.
	bitcoindVersionPrefix = "/Satoshi:"

	// bitcoindVersionSuffix specifies the suffix included in every bitcoind
	// version exposed through GetNetworkInfo.
	bitcoindVersionSuffix = "/"
)

//parsebitcoindversion parse the bitcoind version from its string
//representation.
func parseBitcoindVersion(version string) BackendVersion {
	//trim the version of its prefix and suffix to determine the
	//approriate version number.
	version = strings.TrimPrefix(
		strings.TrimSuffix(version, bitcoindVersionSuffix),
		bitcoindVersionPrefix,
	)
	switch {

	case version < bitcoind19Str:
		return BitcoindPre19
	default:
		return BitcoindPost19
	}

}

// BackendVersion retrieves the version of the backend the client is currently
// connected to.
func (c *Client) BackendVersion() (BackendVersion, error) {
	c.backendVersionMu.Lock()
	defer c.backendVersionMu.Unlock()

	if c.backendVersion != nil {
		return *c.backendVersion, nil
	}

	// We'll start by calling GetInfo. This method doesn't exist for
	// bitcoind nodes as of v0.16.0, so we'll assume the client is connected
	// to a btcd backend if it does exist.
	info, err := c.GetInfo()

	switch err := err.(type) {
	// Parse the btcd version and cache it.
	case nil:
		log.Debugf("Detected btcd version: %v", info.Version)
		version := Btcd
		c.backendVersion = &version
		return *c.backendVersion, nil

	// Inspect the RPC error to ensure the method was not found, otherwise
	// we actually ran into an error.
	case *btcjson.RPCError:
		if err.Code != btcjson.ErrRPCMethodNotFound.Code {
			return 0, fmt.Errorf("unable to detect btcd version: "+
				"%v", err)
		}

	default:
		return 0, fmt.Errorf("unable to detect btcd version: %v", err)
	}

	// Since the GetInfo method was not found, we assume the client is
	// connected to a bitcoind backend, which exposes its version through
	// GetNetworkInfo.
	networkInfo, err := c.GetNetworkInfo()
	if err != nil {
		return 0, fmt.Errorf("unable to detect bitcoind version: %v", err)
	}

	// Parse the bitcoind version and cache it.
	log.Debugf("Detected bitcoind version: %v", networkInfo.SubVersion)
	version := parseBitcoindVersion(networkInfo.SubVersion)
	c.backendVersion = &version

	return *c.backendVersion, nil
}

//new creates a new RPC client based on the provided connection
//configrration details ,the notification hanlders parameter may be nil
//if you are not interested in receiving notifications and will be
//ignored if the configuation is set to run in http post mode.
func New(config *ConnConfig, ntfnHandlers *NotificationHandlers) (*Client, error) {

	var wsConn *websocket.Conn
	var httpClient *http.Client
	connEstablished := make(chan struct{})

	var start bool
	if config.HTTPPostMode {
		ntfnHandlers = nil
		start

		var err error
		httpClient, err = newHTTPClient(config)
		if err != nil {
			return nil, err
		}
	} else {
		if !config.DisableConnectOnNew {
			var err error
			wsConn, err = dial(config)
			if err != nil {
				return nil, err
			}
			start = true
		}

	}

	client := &Client{
		config:          config,
		wsConn:          wsConn,
		httpClient:      httpClient,
		requestMap:      make(map[uint64]*list.Element),
		requestList:     list.New(),
		ntfnHandlers:    ntfnHandlers,
		ntfnState:       newNotificationState(),
		sendChan:        make(chan []byte, sendBufferSize),
		sendPostChan:    make(chan *sendPostDetails, sendPostBufferSize),
		connEstablished: connEstablished,
		disconnect:      make(chan struct{}),
		shutdown:        make(chan struct{}),
	}

	//difault network is mainnet ,no parameters are necessary but if miannet
	//is specified it will be the param
	switch config.Params {
	case "":
		fallthrough
	case chaincfg.MainNetParams.Name:
		client.chainParams = &chaincfg.MainNetParams
	case chaincfg.TestNet3Params.Name:
		client.chainParams = &chaincfg.TestNet3Params
	case chaincfg.RegressionNetParams.Name:
		client.chainParams = &chaincfg.RegressionNetParams
	case chaincfg.SimNetParams.Name:
		client.chainParams = &chaincfg.SimNetParams
	default:
		return nil, fmt.Errorf("rpcclient.New: Unknown chain %s", config.Params)

	}

	if start {
		log.Infof("established connection to RPC server %s", config.Host)
		close(connEstablished)
		client.start()
		if !client.config.HTTPPostMode && !client.config.DisableAutoReconnect {
			client.wg.Add(1)
			go client.wsReconnectHandler()
		}
	}

	return client, nil

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
