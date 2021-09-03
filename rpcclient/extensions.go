package rpcclient

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/btcsuite/btcutil"
)

//futuredebuglevel is a future promised to deliver the result of a
//debuglevelasync PRC invocation (or an application error)
type FutureDebugLevelResult chan *response

//receive waits for the response promised by the future and returns the result
//of setting the debug logging level to the passed level specification or the
//list of the available subsystems for the special keyword "show"
func(r FutureDebugLevelResult) Receive()(string,error){
	res,err := receiveFuture(r)
	if err != nil {
		return "",err
	}

	//unmashall the result as a string
	var result string
	err = json.Unmarshal(res,&result)
	if err != nil {
		return "",err
	}

	return result,nil
}

//debuglevelasync returns an instance of a type that can be used to get the
//result of the RPC at some future time by invoking the receive fucntion on
//the returned instance .


//see Debuglevel for the blocking version and more details.
func(c *Client) DebugLevelAsync(levelSpec string)FutureDebugLevelResult{
	cmd := btcjson.NewDebugLevelCmd(levelSpec)
	return c.sendCmd(cmd)
}


// FutureCreateEncryptedWalletResult is a future promise to deliver the error
// result of a CreateEncryptedWalletAsync RPC invocation.
type FutureCreateEncryptedWalletResult chan *response

// Receive waits for and returns the error response promised by the future.
func (r FutureCreateEncryptedWalletResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// CreateEncryptedWalletAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See CreateEncryptedWallet for the blocking version and more details.
//
// NOTE: This is a btcwallet extension.
func (c *Client) CreateEncryptedWalletAsync(passphrase string) FutureCreateEncryptedWalletResult {
	cmd := btcjson.NewCreateEncryptedWalletCmd(passphrase)
	return c.sendCmd(cmd)
}

// CreateEncryptedWallet requests the creation of an encrypted wallet.  Wallets
// managed by btcwallet are only written to disk with encrypted private keys,
// and generating wallets on the fly is impossible as it requires user input for
// the encryption passphrase.  This RPC specifies the passphrase and instructs
// the wallet creation.  This may error if a wallet is already opened, or the
// new wallet cannot be written to disk.
//
// NOTE: This is a btcwallet extension.
func (c *Client) CreateEncryptedWallet(passphrase string) error {
	return c.CreateEncryptedWalletAsync(passphrase).Receive()
}

//futurelistaddrsstransactionresult is a future promise to deliver the result
//of a listaddresstrnasactionasync rpc invocation(or an applicable error)
type FutureListAddressTransactionResult chan *response

//recive waits for the response promised by the future and retusn information
//about all trnasaction associated with the provided addresses.
func(r FutureListAddressTransactionResult)Receive()([]btcjson.ListTransactionsResult,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,err
	}

	//unmarshal the result as an array of listtransaction objects.
	var transactions []btcjson.ListTransactionsResult
	err = json.Unmarshal(res,&transactions)
	if err != nil {
		return nil,err
	}

	return transactions,nil
}

func(c *Client)ListAddressTransactionsAsync(address []btcutil.Address,account string) FutureListAddressTransactionResult{

	//convert address to strings.
	addrs := make([]string,0,len(address))
	for _,addr := range address{
		addrs = append(addrs,addr.EncodeAddress())

	}
	cmd := btcjson.NewListAddressTransactionsCmd(addrs, &account)
	return c.sendCmd(cmd)

}

//futuregetbestblockresult is  a future promise to deliver the result of a
//getbestblockasync rpc invoaction (or an application error)
type FutureGetBestBlockResult chan *response

//receive wait for the respose promised by the future and returns the hash
//and height of the block in the logest(best)chain.
func(r FutureGetBestBlockResult) Receive()(*chainhash.Hash,int32,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,0,err
	}

	//unmarshal result as a getbestblock result object
	var bestBlock btcjson.GetBestBlockResult
	err = json.Unmarshal(res,&bestBlock)
	if err != nil {
		return nil, 0, err
	}

	//convert to hash from string
	hash ,err := chainhash.NewHashFromStr(bestBlock.Hash)
	if err != nil {
		return nil,0,err
	}
	return hash,bestBlock.Height,nil

}

// GetBestBlockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBestBlock for the blocking version and more details.
//
// NOTE: This is a btcd extension.
func (c *Client) GetBestBlockAsync() FutureGetBestBlockResult {
	cmd := btcjson.NewGetBestBlockCmd()
	return c.sendCmd(cmd)
}

// GetBestBlock returns the hash and height of the block in the longest (best)
// chain.
//
// NOTE: This is a btcd extension.
func (c *Client) GetBestBlock() (*chainhash.Hash, int32, error) {
	return c.GetBestBlockAsync().Receive()
}

// FutureGetCurrentNetResult is a future promise to deliver the result of a
// GetCurrentNetAsync RPC invocation (or an applicable error).
type FutureGetCurrentNetResult chan *response

// Receive waits for the response promised by the future and returns the network
// the server is running on.
func (r FutureGetCurrentNetResult) Receive() (wire.BitcoinNet, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as an int64.
	var net int64
	err = json.Unmarshal(res, &net)
	if err != nil {
		return 0, err
	}

	return wire.BitcoinNet(net), nil
}

func (c *Client) GetCurrentNetAsync() FutureGetCurrentNetResult {
	cmd := btcjson.NewGetCurrentNetCmd()
	return c.sendCmd(cmd)
}

// GetCurrentNet returns the network the server is running on.
//
// NOTE: This is a btcd extension.
func (c *Client) GetCurrentNet() (wire.BitcoinNet, error) {
	return c.GetCurrentNetAsync().Receive()
}

type FutureGetHeadersResult chan *response

// Receive waits for the response promised by the future and returns the
// getheaders result.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (r FutureGetHeadersResult) Receive() ([]wire.BlockHeader, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a slice of strings.
	var result []string
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}

	// Deserialize the []string into []wire.BlockHeader.
	headers := make([]wire.BlockHeader, len(result))
	for i, headerHex := range result {
		serialized, err := hex.DecodeString(headerHex)
		if err != nil {
			return nil, err
		}
		err = headers[i].Deserialize(bytes.NewReader(serialized))
		if err != nil {
			return nil, err
		}
	}
	return headers, nil
}

//getheaderasnync returns a instance of a type that be used toget /
//the rusult of the Rpc at some future time by invoking the receiver fucntion
//on the returned instance
func(c *Client) GetHeadersAsync(blockLocators []chainhash.Hash,hashStop *chainhash.Hash) FutureGetBlockResult {
	locators := make([]string ,len(blockLocators))
	for i := range blockLocators{
		locators[i] = blockLocators[i].String()

	}

	hash := ""
	if hashStop != nil {
		hash = hashStop.String()

	}
	cmd  := btcjson.NewGetHeadersCmd(locators,hash)
	return c.sendCmd(cmd)
}

// GetHeaders mimics the wire protocol getheaders and headers messages by
// returning all headers on the main chain after the first known block in the
// locators, up until a block hash matches hashStop.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) GetHeaders(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) ([]wire.BlockHeader, error) {
	return c.GetHeadersAsync(blockLocators, hashStop).Receive()
}




















