package rpcclient

import (
	"BtcoinProject/btcjson"
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"bytes"
	"encoding/hex"
	"encoding/json"
)

//futuregetbestblockhashresult is a future promise to deliever the result of
//a getbestblockasync rpc invocation (or an applicable error)
type FutureGetBestBlockHashResult chan *response

//receiver waits for the response promised by the future and returns the hsh of
//the best blcok in the longest blcok chain,
func(r FutureGetBestBlockHashResult) Receive() (*chainhash.Hash,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,err
	}

	//unmarshal result as a string
	var txHashStr string
	err  = json.Unmarshal(res,&txHashStr)
	if err != nil {
		return nil ,err
	}

	return chainhash.NewHashFromStr(txHashStr)
}
// GetBestBlockHashAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBestBlockHash for the blocking version and more details.
func (c *Client) GetBestBlockHashAsync() FutureGetBestBlockHashResult {
	cmd := btcjson.NewGetBestBlockHashCmd()
	return c.sendCmd(cmd)
}

// GetBestBlockHash returns the hash of the best block in the longest block
// chain.
func (c *Client) GetBestBlockHash() (*chainhash.Hash, error) {
	return c.GetBestBlockHashAsync().Receive()
}

// FutureGetBlockResult is a future promise to deliver the result of a
// GetBlockAsync RPC invocation (or an applicable error).
type FutureGetBlockResult chan *response

//futuregetblockresult if a future promise to deliver the result of a
//getblockasyc PRC invocation (or an application error)
func (r FutureGetBlockResult) Receive() (*wire.MsgBlock,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,err
	}

	//unmarshall result as a string
	var blockHex string
	err = json.Unmarshal(res,&blockHex)
	if err != nil {
		return nil ,err
	}

	//decode the serialized block hex to raw bytes.
	serializedBlock,err := hex.DecodeString(blockHex)
	if err != nil {
		return nil,err
	}


	//deserialize the block and return it.
	var msgBlock wire.MsgBlock
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil ,err
	}
	return &msgBlock,nil
}

// GetBlockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBlock for the blocking version and more details.
func (c *Client) GetBlockAsync(blockHash *chainhash.Hash) FutureGetBlockResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetBlockCmd(hash, nil)
	return c.sendCmd(cmd)
}

// GetBlock returns a raw block from the server given its hash.
//
// See GetBlockVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	return c.GetBlockAsync(blockHash).Receive()
}

// FutureGetBlockVerboseResult is a future promise to deliver the result of a
// GetBlockVerboseAsync RPC invocation (or an applicable error).
type FutureGetBlockVerboseResult chan *response


//receive waits for the response pormised by the future and reutrns the data
//structre form the server with information about the request block
func(r FutureGetBlockVerboseResult) Receive()(*btcjson.GetBlockVerboseResult,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,err
	}

	//unmarshall the raw result into a blockResult
	var blockResult btcjson.GetBlockVerboseResult
	err = json.Unmarshal(res,&blockResult)
	if err != nil {
		return nil ,err
	}

	return &blockResult,nil
}

//getblockverboseAsync returns an instanse fo a type that can be used to get
//the result of the Rpc at some future time by invoking the rcevie
//function on the returned instance
func(c *Client)GetBlockVerboseAsync(blockHash *chainhash.Hash) FutureGetBlockVerboseResult{
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}
	//from the bitcoin-cli getblock deoucmentation:
	//if verbosity is 1 ,returns an object with information about block.
	cmd := btcjson.NewGetBlockCmd(hash, btcjson.Int(1))
	return c.sendCmd(cmd)
}

// GetBlockVerbose returns a data structure from the server with information
// about a block given its hash.
//
// See GetBlockVerboseTx to retrieve transaction data structures as well.
// See GetBlock to retrieve a raw block instead.
func (c *Client) GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	return c.GetBlockVerboseAsync(blockHash).Receive()
}

type FutureGetBlockVerboseTxResult chan *response

func (r FutureGetBlockVerboseTxResult) Receive() (*btcjson.GetBlockVerboseTxResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var blockResult btcjson.GetBlockVerboseTxResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}

	return &blockResult, nil
}

//getblockverbosetxAsync returns an instance of a type can be used to get
//the result of the rpc at some future time by invoking the receive function
//on the reutrned instance.
func(c *Client)GetBlockVerboseTxAsync(blockHash *chainhash.Hash) FutureGetBlockVerboseTxResult{
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}
	//from the bitcoin_cli gerblock documentation:
	//if verbosity is 2 ,rueturns an object with information about ....
	cmd := btcjson.NewGetBlockCmd(hash, btcjson.Int(2))

	return c.sendCmd(cmd)
}

// GetBlockVerboseTx returns a data structure from the server with information
// about a block and its transactions given its hash.
//
// See GetBlockVerbose if only transaction hashes are preferred.
// See GetBlock to retrieve a raw block instead.
func (c *Client) GetBlockVerboseTx(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseTxResult, error) {
	return c.GetBlockVerboseTxAsync(blockHash).Receive()
}

// FutureGetBlockCountResult is a future promise to deliver the result of a
// GetBlockCountAsync RPC invocation (or an applicable error).
type FutureGetBlockCountResult chan *response

//receive waits for the response pormised by the future and reutns the number
//of blocks in the longest block chain.
func(r FutureGetBlockCountResult)Receive()(int64 ,error){
	res,err := receiveFuture(r)
	if err != nil {
		return 0,err
	}

	//unmarshall the reuslt as an int64
	var count int64
	err = json.Unmarshal(res,&count)
	if err != nil {
		return 0,err
	}
	return count,nil
}

// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBlockCount for the blocking version and more details.
func (c *Client) GetBlockCountAsync() FutureGetBlockCountResult {
	cmd := btcjson.NewGetBlockCountCmd()
	return c.sendCmd(cmd)
}

// GetBlockCount returns the number of blocks in the longest block chain.
func (c *Client) GetBlockCount() (int64, error) {
	return c.GetBlockCountAsync().Receive()
}


























