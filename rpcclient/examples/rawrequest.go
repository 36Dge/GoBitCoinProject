package rpcclient

import (
	"encoding/json"
)

//futureRawResult is a future promise to deliever the result of a RawRequest PRC
//invocation (or an applicable error)
type FutureRawResult chan *response

//receive waits for the response promised by the future and returns the raw
//response,or an error if the request was unsuccessful.
func (r FutureRawResult) Receive() (json.RawMessage, error) {
	return receiveFuture(r)
}
