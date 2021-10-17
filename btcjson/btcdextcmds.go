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

//newnodecmd returns a new instance which can be used to inssue a `node`
//json-rpc command.
//the parameters which are points indicate they are optical .passing nil
//for optional parameters will use the defualt value.
func NewNodeCmd (subCmd NodeSubCmd,target string,connectSubCmd *string) *NodeCmd {
	return &NodeCmd{
		SubCmd:subCmd,
		Target:target,
		ConnectSubCmd:connectSubCmd,
	}
}

//debuglevelcmd defines the debuglevel json_rpc command.this command is not a
//standard bitcoin command.it is an extension for btcd.
type DebugLevelCmd struct {
	levelSpec string
}

//newdebuglevelcmd returns a new debuglevelcmd which can be used to issue t a
//debuglevel json_rpc command .this comand is not a standard bitcoin cammand.
//it is an extension for btcd.
func NewDebugLevelCmd(levelSpec string) *DebugLevelCmd {
	return &DebugLevelCmd{levelSpec: levelSpec}
}

//generatetoaddaddresscmd defines the >.>JSON_PRC command.
type GenerateToAddressCmd struct {
	NumBlocks int64
	Address string
	MaxTries *int64 `jsonrpcdefault:"1000000"`
}

// NewGenerateToAddressCmd returns a new instance which can be used to issue a
// generatetoaddress JSON-RPC command.
func NewGenerateToAddressCmd(numBlocks int64, address string, maxTries *int64) *GenerateToAddressCmd {
	return &GenerateToAddressCmd{
		NumBlocks: numBlocks,
		Address:   address,
		MaxTries:  maxTries,
	}
}

//getbestblockcmd define the getbestblock json-rpc command.
type GetBestBlockCmd struct {

}


//netgetbestblockcmd returns a new instance which can be used to issue  a
//getbestblock json_RPC command.
func NewGetBestBlockCmd () *GetBestBlockCmd {
	return &GetBestBlockCmd{}
}



// GetCurrentNetCmd defines the getcurrentnet JSON-RPC command.
type GetCurrentNetCmd struct{}

// NewGetCurrentNetCmd returns a new instance which can be used to issue a
// getcurrentnet JSON-RPC command.
func NewGetCurrentNetCmd() *GetCurrentNetCmd {
	return &GetCurrentNetCmd{}
}

type GetHeadersCmd struct {
	BlockLocators []string `json:"blockLocators"`
	HashStop string `json:"hashstop"`
}

// NewGetHeadersCmd returns a new instance which can be used to issue a
// getheaders JSON-RPC command.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrd/dcrjson.
func NewGetHeadersCmd(blockLocators []string, hashStop string) *GetHeadersCmd {
	return &GetHeadersCmd{
		BlockLocators: blockLocators,
		HashStop:      hashStop,
	}
}

// VersionCmd defines the version JSON-RPC command.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrd/dcrjson.
type VersionCmd struct{}

// NewVersionCmd returns a new instance which can be used to issue a JSON-RPC
// version command.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrd/dcrjson.
func NewVersionCmd() *VersionCmd { return new(VersionCmd) }


func init() {
	//no special flags for commands in this file.
	flags := UsageFlag(0)

	MustRegisterCmd("debuglevel", (*DebugLevelCmd)(nil), flags)
	MustRegisterCmd("node", (*NodeCmd)(nil), flags)
	MustRegisterCmd("generate", (*GenerateCmd)(nil), flags)
	MustRegisterCmd("generatetoaddress", (*GenerateToAddressCmd)(nil), flags)
	MustRegisterCmd("getbestblock", (*GetBestBlockCmd)(nil), flags)
	MustRegisterCmd("getcurrentnet", (*GetCurrentNetCmd)(nil), flags)
	MustRegisterCmd("getheaders", (*GetHeadersCmd)(nil), flags)
	MustRegisterCmd("version", (*VersionCmd)(nil), flags)
}




//over

