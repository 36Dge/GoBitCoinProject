package btcjson

//createnewaccount defines the createnewaccount json-Rpc command.
type CreateNewAccountCmd struct {
	Account string
}

//returnes a new instance which can be used to issue a createnewaccount
//json-RPC command.
func NewCreateNewAccountCmd(account string) *CreateNewAccountCmd {
	return &CreateNewAccountCmd{Account: account}
}

//define the dumpwallet josn-RPC command.
type DumpWalletCmd struct {
	Filename string
}

//returns a new instance which can be used to issue a dumpwallet json_rpc
//command.
func NewDumpWalletCmd(filename string) *DumpWalletCmd {
	return &DumpWalletCmd{Filename: filename}
}

//defines the importaddress json_rpc command.
type ImportAddressCmd struct {
	Address string
	Account string
	Rescan  *bool `jsonrpcdefault:"true"`
}

//newimportaddresscmd returns a new instance which can be used to inssue an
//importaddress json-rpc command.

func NewImportAddressCmd(address string, account string, rescan *bool) *ImportAddressCmd {
	return &ImportAddressCmd{
		Address: address,
		Account: account,
		Rescan:  rescan,
	}
}

//importpubkeycmd defines the importpubkey json-rpc command.
type ImportPubKeyCmd struct {
	PubKey string
	Rescan *bool `jsonrpcdefault:"true"`

}


// NewImportPubKeyCmd returns a new instance which can be used to issue an
// importpubkey JSON-RPC command.
func NewImportPubKeyCmd(pubKey string, rescan *bool) *ImportPubKeyCmd {
	return &ImportPubKeyCmd{
		PubKey: pubKey,
		Rescan: rescan,
	}
}


// RenameAccountCmd defines the renameaccount JSON-RPC command.
type RenameAccountCmd struct {
	OldAccount string
	NewAccount string
}

// NewRenameAccountCmd returns a new instance which can be used to issue a
// renameaccount JSON-RPC command.
func NewRenameAccountCmd(oldAccount, newAccount string) *RenameAccountCmd {
	return &RenameAccountCmd{
		OldAccount: oldAccount,
		NewAccount: newAccount,
	}
}

func init() {
	// The commands in this file are only usable with a wallet server.
	flags := UFWalletOnly

	MustRegisterCmd("createnewaccount", (*CreateNewAccountCmd)(nil), flags)
	MustRegisterCmd("dumpwallet", (*DumpWalletCmd)(nil), flags)
	MustRegisterCmd("importaddress", (*ImportAddressCmd)(nil), flags)
	MustRegisterCmd("importpubkey", (*ImportPubKeyCmd)(nil), flags)
	MustRegisterCmd("importwallet", (*ImportWalletCmd)(nil), flags)
	MustRegisterCmd("renameaccount", (*RenameAccountCmd)(nil), flags)
}


//over

















