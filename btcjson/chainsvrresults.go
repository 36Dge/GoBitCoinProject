package btcjson

// GetRawMempoolVerboseResult models the data returned from the getrawmempool
// command when the verbose flag is set.  When the verbose flag is not set,
// getrawmempool returns an array of transaction hashes.
type GetRawMempoolVerboseResult struct {
	Size             int32    `json:"size"`
	Vsize            int32    `json:"vsize"`
	Weight           int32    `json:"weight"`
	Fee              float64  `json:"fee"`
	Time             int64    `json:"time"`
	Height           int64    `json:"height"`
	StartingPriority float64  `json:"startingpriority"`
	CurrentPriority  float64  `json:"currentpriority"`
	Depends          []string `json:"depends"`
}

// GetBlockVerboseResult models the data from the getblock command when the
// verbose flag is set to 1.  When the verbose flag is set to 0, getblock returns a
// hex-encoded string. When the verbose flag is set to 1, getblock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getblock returns an object whose tx field is an array of raw transactions.
// Use GetBlockVerboseTxResult to unmarshal data received from passing verbose=2 to getblock.
type GetBlockVerboseResult struct {
	Hash          string        `json:"hash"`
	Confirmations int64         `json:"confirmations"`
	StrippedSize  int32         `json:"strippedsize"`
	Size          int32         `json:"size"`
	Weight        int32         `json:"weight"`
	Height        int64         `json:"height"`
	Version       int32         `json:"version"`
	VersionHex    string        `json:"versionHex"`
	MerkleRoot    string        `json:"merkleroot"`
	Tx            []string      `json:"tx,omitempty"`
	RawTx         []TxRawResult `json:"rawtx,omitempty"` // Note: this field is always empty when verbose != 2.
	Time          int64         `json:"time"`
	Nonce         uint32        `json:"nonce"`
	Bits          string        `json:"bits"`
	Difficulty    float64       `json:"difficulty"`
	PreviousHash  string        `json:"previousblockhash"`
	NextHash      string        `json:"nextblockhash,omitempty"`
}

// GetBlockVerboseTxResult models the data from the getblock command when the
// verbose flag is set to 2.  When the verbose flag is set to 0, getblock returns a
// hex-encoded string. When the verbose flag is set to 1, getblock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getblock returns an object whose tx field is an array of raw transactions.
// Use GetBlockVerboseResult to unmarshal data received from passing verbose=1 to getblock.
type GetBlockVerboseTxResult struct {
	Hash          string        `json:"hash"`
	Confirmations int64         `json:"confirmations"`
	StrippedSize  int32         `json:"strippedsize"`
	Size          int32         `json:"size"`
	Weight        int32         `json:"weight"`
	Height        int64         `json:"height"`
	Version       int32         `json:"version"`
	VersionHex    string        `json:"versionHex"`
	MerkleRoot    string        `json:"merkleroot"`
	Tx            []TxRawResult `json:"tx,omitempty"`
	Time          int64         `json:"time"`
	Nonce         uint32        `json:"nonce"`
	Bits          string        `json:"bits"`
	Difficulty    float64       `json:"difficulty"`
	PreviousHash  string        `json:"previousblockhash"`
	NextHash      string        `json:"nextblockhash,omitempty"`
}

//getworkresult models the data from the getwork command.
type GetWorkResult struct {
	Data     string `json:"data"`
	Hash1    string `json:"hash_1"`
	Midsdata string `json:"midsdata"`
	Target   string `json:"target"`
}

//infochainresult models the data returned by the chain server.
type InfoChainResult struct {
	Version         int32   `json:"version"`
	ProtocolVersion int32   `json:"protocol_version"`
	Blocks          int32   `json:"blocks"`
	TimeOffset      int32   `json:"time_offset"`
	Connections     int32   `json:"connections"`
	Proxy           string  `json:"proxy"`
	Difficulty      float64 `json:"difficulty"`
	TestNet         bool    `json:"test_net"`
	RelayFee        float64 `json:"relay_fee"`
	Errors          string  `json:"errors"`
}

//txrawresult modes the data from the getrawtrnasaction command.
type TxRawResult struct {
	Hex           string
	Txid          string
	Hash          string
	Size          int32
	Vsize         int32
	Version       int32
	LockTime      uint32
	Vin           []Vin
	Vout          []Vout
	BlockHash     string
	Confirmations uint64
	Time          int64
	Blocktime     int64
}

//txrawdecoderesult model the data from the decoderawtransaction command.
type TxRawDecodeResult struct {
	Txid     string `josn:"txid"`
	Version  int32  `json:"version"`
	Locktime uint32 `json:"locktime"`
	Vin      []Vin  `json:"vin"`
	Vout     []Vout `json:"vout"`
}

//searchrawtrnasactionresult models the date from the serachrawtransaction
//command.
type SearchRawTransactionResult struct {
	Hex           string       `json:"hex,omitempty"`
	Txid          string       `json:"txid"`
	Hash          string       `json:"hash"`
	Size          string       `json:"size"`
	Vsize         string       `json:"vsize"`
	Weight        string       `json:"weight"`
	Version       int32        `json:"version"`
	LockTime      uint32       `json:"lock_time"`
	Vin           []VinPrevOut `json:"vin"`
	Vout          []Vout       `json:"vout"`
	BlockHash     string       `json:"block_hash,omitempty"`
	Confirmations uint64       `json:"confirmations,omitempty"`
	Time          uint64       `json:"time,omitempty"`
	Blocktime     int64        `json:"blocktime,omitempty"`
}

//validateAddressChainResult models the data returned by the chain server
//validateaddress command.
type ValidateAddressChainResult struct {
	isValid bool   `json:"is_valid"`
	Address string `json:"address,omitempty"`
}

//estimatesmartfeeresult modesl the data returned buy the chain server
//estimatesmartfee comand.
type EstimateSmartFeeResult struct {
	FeeRate *float64 `json:"feerate,omitempty"`
	Errors  []string `json:"errors,omitempty"`
	Blocks  int64    `json:"blocks"`
}
