package btcjson

//getblockheaderverboseresult  models the data from the getblockheader command when
//the verbose flag is set ,when the verbose flag is not set ,get blockheader
//returns a hex-encode string
type GetBlockHeaderVerboseResult struct {
	Hash          string  `json:"hash"`
	Confirmations int64   `json:"confirmations"`
	Height        int32   `json:"height"`
	Version       int32   `json:"version"`
	VersionHex    string  `json:"versionhex"`
	MerkleRoot    string  `json:"merkleroot"`
	Time          int64   `json:"time"`
	Nonce         uint64  `json:"nonce"`
	Bits          string  `json:"bits"`
	Difficulty    float64 `json:"difficulty"`
	PreviousHash  string  `json:"previous_hash"`
	NextHash      string  `json:"next_hash"`
}


//getblockstatsresult models the data from the getblockstats commnd.
type GetBlockStatusResult struct {
	AverageFee         int64   `json:"avgfee"`
	AverageFeeRate     int64   `json:"avgfeerate"`
	AverageTxSize      int64   `json:"avgtxsize"`
	FeeratePercentiles []int64 `json:"feerate_percentiles"`
	Hash               string  `json:"blockhash"`
	Height             int64   `json:"height"`
	Ins                int64   `json:"ins"`
	MaxFee             int64   `json:"maxfee"`
	MaxFeeRate         int64   `json:"maxfeerate"`
	MaxTxSize          int64   `json:"maxtxsize"`
	MedianFee          int64   `json:"medianfee"`
	MedianTime         int64   `json:"mediantime"`
	MedianTxSize       int64   `json:"mediantxsize"`
	MinFee             int64   `json:"minfee"`
	MinFeeRate         int64   `json:"minfeerate"`
	MinTxSize          int64   `json:"mintxsize"`
	Outs               int64   `json:"outs"`
	SegWitTotalSize    int64   `json:"swtotal_size"`
	SegWitTotalWeight  int64   `json:"swtotal_weight"`
	SegWitTxs          int64   `json:"swtxs"`
	Subsidy            int64   `json:"subsidy"`
	Time               int64   `json:"time"`
	TotalOut           int64   `json:"total_out"`
	TotalSize          int64   `json:"total_size"`
	TotalWeight        int64   `json:"total_weight"`
	Txs                int64   `json:"txs"`
	UTXOIncrease       int64   `json:"utxo_increase"`
	UTXOSizeIncrease   int64   `json:"utxo_size_inc"`
}

// GetBlockVerboseResult models the data from the getblock command when the
// verbose flag is set to 1.  When the verbose flag is set to 0, getblock returns a
// hex-encoded string. When the verbose flag is set to 1, getblock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getblock returns an object whose tx field is an array of raw transactions.
// Use GetBlockVerboseTxResult to unmarshal data received from passing verbose=2 to getblock.



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


//createMultisigresult models the data returned from the creatmulgising
//command
type CreateMultiSigResult struct {
	Address string `json:"address"`
	RedeemScript string `json:"redeem_script"`
}


//decodeScriptresult models the data returned from the decodescript command.
type DecodeScriptResult struct {
	Asm string `json:"asm"`
	ReqSigs int32 `json:"reqSigs,omitempty"`
	Type string `json:"type"`
	Address []string `json:"address,omitempty"`
	P2sh string `json:"p_2_sh,omitempty"`
}




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


//vout models parts of the tx data.it is defined separately since both
//getrawtrnasaxtion and decoderawtransaction use the same strutere.
type Vout struct {
	Value        float64            `json:"value"`
	N            uint32             `json:"n"`
	ScriptPubKey ScriptPubKeyResult `json:"scriptPubKey"`
}

//prevout represent previous output for an input Vin.
type PrevOut struct {
	Address []string `json:"address,omitempty"`
	Value   float64  `json:"value"`
}

func (v *VinPrevOut) HashWitness() bool {
	return len(v.Coinvbase) > 0
}

// HasWitness returns a bool to show if a Vin has any witness data associated
// with it or not.
func (v *VinPrevOut) HasWitness() bool {
	return len(v.Witness) > 0
}

//vout models parts of the tx data ,it is defined separately since
//both getrawtranction and decoderawtransaction use the same structure.
type Vout struct {
	Value        float64            `json:"value"`
	N            uint32             `json:"n"`
	ScriptPubKey scriptPubKeyResult `json:"scriptPubKey"`
}

//getmininginforesult models the data from the getminnginfo command.
type GetMiningInfoResult struct {
	Blocks             int64   `json:"blocks"`
	CurrentBlockSize   uint64  `json:"current_block_size"`
	CurrentBlockWeight uint64  `json:"current_block_weight"`
	CurrentBlockTx     uint64  `json:"current_block_tx"`
	Difficulty         float64 `json:"difficulty"`
	Errors             string  `json:"errors"`
	Generate           bool    `json:"generate"`
	GenProcLimit       int32   `json:"genproclimit"`
	HashesPerSec       int64   `json:"hashespersec"`
	NetworkHashPS      int64   `json:"network_hash_ps"`
	PooledTx           int64   `json:"pooled_tx"`
	TestNet            bool    `json:"test_net"`
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
