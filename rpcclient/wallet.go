package rpcclient

import "encoding/json"

//futuregettrnasationresult is a future promise to deliver the result of a gettransactionasync PRC
//invocation (or an applicabe error )
type FutureGetTransactionResult chan *response


//receive waits for the response promise by the future and retures detailed
//information about a wallet transaction
func(r FutureGetTransactionResult) Receive() (*btcjson.GetTransactionResult,error){
	res ,err := receiveFuture(r)
	if err != nil {
		return nil ,err
	}

	//unmarshall result as a gettrnanacion result object
	var getTx btcjson.GetTransactionResult
	err = json.Unmarshal(res,&getTx)
	if err != nil {
		return nil ,err
	}

	return &getTx,nil
}
