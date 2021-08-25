package rpcclient

import (
	"BtcoinProject/chaincfg/chainhash"
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