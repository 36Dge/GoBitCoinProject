package blockchain



// NotificationType represents the type of a notification message.
type NotificationType int


// Constants for the type of a notification message.
const (
	// NTBlockAccepted indicates the associated block was accepted into
	// the block chain.  Note that this does not necessarily mean it was
	// added to the main chain.  For that, use NTBlockConnected.
	NTBlockAccepted NotificationType = iota

	// NTBlockConnected indicates the associated block was connected to the
	// main chain.
	NTBlockConnected

	// NTBlockDisconnected indicates the associated block was disconnected
	// from the main chain.
	NTBlockDisconnected
)

// Notification defines notification that is sent to the caller via the callback
// function provided during the call to New and consists of a notification type
// as well as associated data that depends on the type as follows:
// 	- NTBlockAccepted:     *btcutil.Block
// 	- NTBlockConnected:    *btcutil.Block
// 	- NTBlockDisconnected: *btcutil.Block
type Notification struct {
	Type NotificationType
	Data interface{}
}

