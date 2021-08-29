package rpcclient

import "encoding/json"

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



















