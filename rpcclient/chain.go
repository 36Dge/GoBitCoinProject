package rpcclient

import (
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
























