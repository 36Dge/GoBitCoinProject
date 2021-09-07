package rpcclient

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
