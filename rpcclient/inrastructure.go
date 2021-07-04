package rpcclient

import (
	"BtcoinProject/chaincfg"
	"container/list"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	//errIvvalAuth is an error to describe the condition where the client
	//is either unable to authenticate or the specified endpoint is incorrrett.
	ErrInvalidAuth = errors.New("authentication failure")

	//errinvalidendpoint is an error to describe the condition where the
	//websocket handshake failed with specified endpoint
	ErrInvalidEndpoint = errors.New("the endpoint either does not support "+"websocket or does not exist")

	//errClientNotConnected  is an error to descaribe the condition where websocket
	//clente has been created ,but the connection was never established .this condition
	//differs form errclientdisconnect ,which represents an estanlished connection that
	//was lost.
	ErrClientNotConnected = errors.New("the client was never connected")

	ErrClientDisconnect = errors.New("the client has been disconnected")

	ErrClientShutdown = errors.New("the client has been shutdown")

	ErrNotWebsocketClient = errors.New("client is not configured for "+ "websockers")

	ErrClientAlreadyConnected = errors.New("websocket client has already" + "connected")

)

const(
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

