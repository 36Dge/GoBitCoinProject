package rpcclient
/*
package rpccloient implements a websocket-enabled bitcoin jason-rpc client.
overview.

this client provides a robust and easy to use client for interfacing with a
bitcoin PRC server that uses a btcd/bitcoin core compatible bicoin json-rpc
api ,this client has been tested with btcd and bitcoin core.

In addition to the compatible standard HTTP POST JSON-RPC API, btcd and
btcwallet provide a websocket interface that is more efficient than the standard
HTTP POST method of accessing RPC.  The section below discusses the differences
between HTTP POST and websockets.

By default, this client assumes the RPC server supports websockets and has
TLS enabled.  In practice, this currently means it assumes you are talking to
btcd or btcwallet by default.  However, configuration options are provided to
fall back to HTTP POST and disable TLS to support talking with inferior bitcoin
core style RPC servers.





*/