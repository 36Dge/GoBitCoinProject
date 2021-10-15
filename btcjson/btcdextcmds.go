package btcjson

//note:this file is intended to house the PRC commands that are supported by
//a chain server with btcd extenions.

//nodesubcmd defines the type used in the addnode JSON-RPC command for the
//sub command field.
type NodeSubCmd string

const (

	//nconnect indicate the sepcified host that should be connected to .
	NConnect NodeSubCmd =  "connect"

	//NRemove indicates the speecidied peer that should be removed as a
	//persistent peer.
	NRemove NodeSubCmd = "remove"
	//NDdisocnnet indicates the specified peer should be disconnected.
	NDisconnect NodeSubCmd = "disconnect"

)

//nodeCmd defines the dropnode JSON-RPC command.
type NodeCmd struct {
	SubCmd NodeSubCmd `jsonrpcusage:"\"connect|remove|disconnect\""`
	Target string
	ConnectSubCmd *string `jsonrpcusage:"\"perm|temp\""`
}

