package rpcclient

import (
	"BtcoinProject/chaincfg/chainhash"
	"time"
)

var (
	// ErrWebsocketsRequired is an error to describe the condition where the
	// caller is trying to use a websocket-only feature, such as requesting
	// notifications or other websocket requests when the client is
	// configured to run in HTTP POST mode.
	ErrWebsocketsRequired = errors.New("a websocket connection is required " +
		"to use this feature")
)

//notificationState is used to track the current state of successfully
//registered notification so the state can be atuomatically re-eastablish
//reconnect.
type notificationState struct {
	notifyBlocks       bool
	notifyNewTx        bool
	notifyNewTxVerbose bool
	notifyReceived     map[string]struct{}
	notifySpent        map[btcjson.OutPoint]struct{}
}

//copy returns a deep copy of the receice.
func(s *notificationState) Copy()*notificationState{
	var stateCopy notificationState
	stateCopy.notifyBlocks = s.notifyBlocks
	stateCopy.notifyNewTx= s.notifyNewTx
	stateCopy.notifyNewTxVerbose = s.notifyNewTxVerbose
	stateCopy.notifyReceived = make(map[string]struct{})
	for addr := range s.notifyReceived {
		stateCopy.notifyReceived[addr] = struct{}{}

	}

	stateCopy.notifySpent = make(map[btcjson.OutPoint]struct{})
	for op := range s.notifySpent{
		stateCopy.notifySpent[op] = struct{}{}

	}
	return &stateCopy
}

//newnotficationstate returns a  new notification state ready to be populatee.
func newNotificaionState()*notificationState{
	return &notificationState{
		notifyReceived: make(map[string]struct{}),
		notifySpent: make(map[btcjson.OutPoint]struct{}),
	}
}

// newNilFutureResult returns a new future result channel that already has the
// result waiting on the channel with the reply set to nil.  This is useful
// to ignore things such as notifications when the caller didn't specify any
// notification handlers.
func newNilFutureResult() chan *response {
	responseChan := make(chan *response, 1)
	responseChan <- &response{result: nil, err: nil}
	return responseChan
}

type NotificationHandlers struct {
	//onclientconnected is invoked when the client connects or reconnects
	//to the rpc server.this callback is run async with the rest of the
	//notification handlers and is safe for blocking client request.
	OnClientConnected func()

	//onblockconnected is invoked when a block is connected to the longest
	//best chain .it will only be invoked if a preceding call to notifyblocks
	//has been made to register for the notification and the function is non-nil
	//drprecated:use onfilteredblockconnected instead.
	OnBlockConnected func(hash *chainhash.Hash,height int32,t time.Time)

}















