package rpcclient

import "net/http"

//futureRawResult is a future promise to deliever the result of a RawRequest PRC
//invocation (or an applicable error)
type FutureRawResult chan *response

