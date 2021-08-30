package rpcclient

import (
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




















