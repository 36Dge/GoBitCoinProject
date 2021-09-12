package rpcclient

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcutil"
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

	//onfilterblockconnceted is invoked when a block is connected to the
	//longest best chain.it will only be invoked if a preceding call to
	//notifyblocks has been made to register for the nofitication and the functiion
	//and the function is non-nil,its parameter differ from onblockconnected
	//it receiver the block,s height header,and relevant trnasaction.
	OnFliteredBlockConnected func(height int32,header *wire.BlockHeader,txs []*btcutil.Tx)

	//onblockdisconnected is invoked when a block is disconected from the
	//lognest chain.it will only be invoked if a preceding call to
	//notifyBlocks has been made to register for the notiifcation and the
	//function is non-nil
	onBlockDisconnected func(hash *chainhash.Hash,height int32,t time.Time)


	// OnFilteredBlockDisconnected is invoked when a block is disconnected
	// from the longest (best) chain.  It will only be invoked if a
	// preceding NotifyBlocks has been made to register for the notification
	// and the call to function is non-nil.  Its parameters differ from
	// OnBlockDisconnected: it receives the block's height and header.
	OnFilteredBlockDisconnected func(height int32, header *wire.BlockHeader)


	// OnRecvTx is invoked when a transaction that receives funds to a
	// registered address is received into the memory pool and also
	// connected to the longest (best) chain.  It will only be invoked if a
	// preceding call to NotifyReceived, Rescan, or RescanEndHeight has been
	// made to register for the notification and the function is non-nil.
	//
	// Deprecated: Use OnRelevantTxAccepted instead.
	OnRecvTx func(transaction *btcutil.Tx, details *btcjson.BlockDetails)


	// OnRedeemingTx is invoked when a transaction that spends a registered
	// outpoint is received into the memory pool and also connected to the
	// longest (best) chain.  It will only be invoked if a preceding call to
	// NotifySpent, Rescan, or RescanEndHeight has been made to register for
	// the notification and the function is non-nil.
	//
	// NOTE: The NotifyReceived will automatically register notifications
	// for the outpoints that are now "owned" as a result of receiving
	// funds to the registered addresses.  This means it is possible for
	// this to invoked indirectly as the result of a NotifyReceived call.
	//
	// Deprecated: Use OnRelevantTxAccepted instead.
	OnRedeemingTx func(transaction *btcutil.Tx, details *btcjson.BlockDetails)

	// OnRelevantTxAccepted is invoked when an unmined transaction passes
	// the client's transaction filter.
	//
	// NOTE: This is a btcsuite extension ported from
	// github.com/decred/dcrrpcclient.
	OnRelevantTxAccepted func(transaction []byte)

	// OnRescanFinished is invoked after a rescan finishes due to a previous
	// call to Rescan or RescanEndHeight.  Finished rescans should be
	// signaled on this notification, rather than relying on the return
	// result of a rescan request, due to how btcd may send various rescan
	// notifications after the rescan request has already returned.
	//
	// Deprecated: Not used with RescanBlocks.
	OnRescanFinished func(hash *chainhash.Hash, height int32, blkTime time.Time)

	// OnRescanProgress is invoked periodically when a rescan is underway.
	// It will only be invoked if a preceding call to Rescan or
	// RescanEndHeight has been made and the function is non-nil.
	//
	// Deprecated: Not used with RescanBlocks.
	OnRescanProgress func(hash *chainhash.Hash, height int32, blkTime time.Time)

	// OnTxAccepted is invoked when a transaction is accepted into the
	// memory pool.  It will only be invoked if a preceding call to
	// NotifyNewTransactions with the verbose flag set to false has been
	// made to register for the notification and the function is non-nil.
	OnTxAccepted func(hash *chainhash.Hash, amount btcutil.Amount)

	// OnTxAccepted is invoked when a transaction is accepted into the
	// memory pool.  It will only be invoked if a preceding call to
	// NotifyNewTransactions with the verbose flag set to true has been
	// made to register for the notification and the function is non-nil.
	OnTxAcceptedVerbose func(txDetails *btcjson.TxRawResult)

	// OnBtcdConnected is invoked when a wallet connects or disconnects from
	// btcd.
	//
	// This will only be available when client is connected to a wallet
	// server such as btcwallet.
	OnBtcdConnected func(connected bool)

	// OnAccountBalance is invoked with account balance updates.
	//
	// This will only be available when speaking to a wallet server
	// such as btcwallet.
	OnAccountBalance func(account string, balance btcutil.Amount, confirmed bool)

	// OnWalletLockState is invoked when a wallet is locked or unlocked.
	//
	// This will only be available when client is connected to a wallet
	// server such as btcwallet.
	OnWalletLockState func(locked bool)

	// OnUnknownNotification is invoked when an unrecognized notification
	// is received.  This typically means the notification handling code
	// for this package needs to be updated for a new notification type or
	// the caller is using a custom notification this package does not know
	// about.
	OnUnknownNotification func(method string, params []json.RawMessage)

}

type wrongNumParams int

//error satisfies the builtin error interface.
func(e wrongNumParams) Error()string {
	return fmt.Sprintf("wrong number of parameters(%d)",e)
}


// NotifyBlocks registers the client to receive notifications when blocks are
// connected and disconnected from the main chain.  The notifications are
// delivered to the notification handlers associated with the client.  Calling
// this function has no effect if there are no notification handlers and will
// result in an error if the client is configured to run in HTTP POST mode.
//
// The notifications delivered as a result of this call will be via one of
// OnBlockConnected or OnBlockDisconnected.
//
// NOTE: This is a btcd extension and requires a websocket connection.
func (c *Client) NotifyBlocks() error {
	return c.NotifyBlocksAsync().Receive()
}

// FutureNotifySpentResult is a future promise to deliver the result of a
// NotifySpentAsync RPC invocation (or an applicable error).
//
// Deprecated: Use FutureLoadTxFilterResult instead.
type FutureNotifySpentResult chan *response

// Receive waits for the response promised by the future and returns an error
// if the registration was not successful.
func (r FutureNotifySpentResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}















