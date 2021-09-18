package rpcclient

import (
	"encoding/json"
	"errors"
)

//futureRawResult is a future promise to deliever the result of a RawRequest PRC
//invocation (or an applicable error)
type FutureRawResult chan *response

//receive waits for the response promised by the future and returns the raw
//response,or an error if the request was unsuccessful.
func (r FutureRawResult) Receive() (json.RawMessage, error) {
	return receiveFuture(r)
}

//rawrequestasync returns an instance of a type that can be used to get the
//result of a costom RPC request at some future time by invoking the receive
//function on the returned instance.
func (c *Client) RawRequestAsync(method string, params []json.RawMessage) FutureRawResult {
	//method may not be empty.
	if method == " " {
		return newFutureError(errors.New("no method"))
	}

	//marshal parameters as "[]" instead of "null"when no paramaters
	//are passed.
	if params == nil {
		params = []json.RawMessage{}
	}

	//create a raw JSON-RPC request using the provided method and params
	//and marshall it .this if done rather than using the sendCmd function
	//since that relies on marshalling registered btcjson commands rather
	//than custom commands.
	id := c.NextID()
	rawRequest := &btcjson.Request{
		Jsonrpc: "1.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	marshalledJSON, err := json.Marshal(rawRequest)
	if err != nil {
		return newFutureError(err)
	}

	//generate the request and send it along with a channel to respond on.
	responseChan := make(chan *response, 1)
	jReq := &jsonRequest{
		id:             id,
		method:         method,
		cmd:            nil,
		marshalledJSON: marshalledJSON,
		responseChan:   responseChan,
	}
	c.sendRequest(jReq)
	return responseChan

}
// RawRequest allows the caller to send a raw or custom request to the server.
// This method may be used to send and receive requests and responses for
// requests that are not handled by this client package, or to proxy partially
// unmarshaled requests to another JSON-RPC server if a request cannot be
// handled directly.
func (c *Client) RawRequest(method string, params []json.RawMessage) (json.RawMessage, error) {
	return c.RawRequestAsync(method, params).Receive()
}

//over