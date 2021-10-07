package rpcclient

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/wire"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/btcsuite/btcutil"
)

const (
	//defaultMaxfeerate is the default maximum fee rate in sat/kb enforced
	//by bitcoinv0.19.0 or after for trnasaction broadcast
	defaultMaxFeeRate  = btcutil.SatoshiPerBitcent / 10

)

//signhashtype enumerates the available singature hashing type s
//that the singRawtransaction function accepts.
type SigHashType string

//constants used to indicate the sinnature hash type for singrawtrnasaction
const (
	//sighshall indicates all of outputs should be singed.
	SigHash SigHashType = "ALL"

	//sighashnone indicate none of the outputs shoulbe be sined .this
	//can be thought of as specifying the singer does not care where the
	//bitcons go.
	SigHashNone SigHashType = "NONE"
	// SigHashSingle indicates that a SINGLE output should be signed.  This
	// can be thought of specifying the signer only cares about where ONE of
	// the outputs goes, but not any of the others.
	SigHashSingle SigHashType = "SINGLE"

	// SigHashAllAnyoneCanPay indicates that signer does not care where the
	// other inputs to the transaction come from, so it allows other people
	// to add inputs.  In addition, it uses the SigHashAll signing method
	// for outputs.
	SigHashAllAnyoneCanPay SigHashType = "ALL|ANYONECANPAY"

	// SigHashNoneAnyoneCanPay indicates that signer does not care where the
	// other inputs to the transaction come from, so it allows other people
	// to add inputs.  In addition, it uses the SigHashNone signing method
	// for outputs.
	SigHashNoneAnyoneCanPay SigHashType = "NONE|ANYONECANPAY"

	// SigHashSingleAnyoneCanPay indicates that signer does not care where
	// the other inputs to the transaction come from, so it allows other
	// people to add inputs.  In addition, it uses the SigHashSingle signing
	// method for outputs.
	SigHashSingleAnyoneCanPay SigHashType = "SINGLE|ANYONECANPAY"

)


//string returns the signHashtype in human-readable form.
func(s SigHashType) String()string{
	return string(s)
}


type FutureGetRawTransactionResult chan *response

//receive waits for the response pormised by the future and returnas a
//transaction given its hash.
func (r FutureGetRawTransactionResult) Receive()(*btcutil.Tx,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil ,err
	}

	//unmrashall result as a string
	var txHex string
	err = json.Unmarshal(res,&txHex)
	if err != nil {
		return nil ,err
	}

	//decode the serialized trnasacion hex to raw bytes.
	serializedTx ,err := hex.DecodeString(txHex)
	if err != nil {
		return nil ,err
	}

	//deserialize the trnasaction and return it .
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(serializedTx));err != nil{
		return nil,err
	}

	return btcutil.NewTx(&msgTx),nil
}

//getrawtrnactionasync returns an insatnace of a type that can be used to get
//the result of the rpc at some future time by invoking the receive function
//the returned instance.

// See GetRawTransaction for the blocking version and more details.
func (c *Client) GetRawTransactionAsync(txHash *chainhash.Hash) FutureGetRawTransactionResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := btcjson.NewGetRawTransactionCmd(hash, btcjson.Int(0))
	return c.sendCmd(cmd)
}

// GetRawTransaction returns a transaction given its hash.
//
// See GetRawTransactionVerbose to obtain additional information about the
// transaction.
func (c *Client) GetRawTransaction(txHash *chainhash.Hash) (*btcutil.Tx, error) {
	return c.GetRawTransactionAsync(txHash).Receive()
}
// FutureGetRawTransactionVerboseResult is a future promise to deliver the
// result of a GetRawTransactionVerboseAsync RPC invocation (or an applicable
// error).
type FutureGetRawTransactionVerboseResult chan *response

// Receive waits for the response promised by the future and returns information
// about a transaction given its hash.
func (r FutureGetRawTransactionVerboseResult) Receive() (*btcjson.TxRawResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a gettrawtransaction result object.
	var rawTxResult btcjson.TxRawResult
	err = json.Unmarshal(res, &rawTxResult)
	if err != nil {
		return nil, err
	}

	return &rawTxResult, nil
}

// GetRawTransactionVerboseAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetRawTransactionVerbose for the blocking version and more details.
func (c *Client) GetRawTransactionVerboseAsync(txHash *chainhash.Hash) FutureGetRawTransactionVerboseResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := btcjson.NewGetRawTransactionCmd(hash, btcjson.Int(1))
	return c.sendCmd(cmd)
}

// GetRawTransactionVerbose returns information about a transaction given
// its hash.
//
// See GetRawTransaction to obtain only the transaction already deserialized.
func (c *Client) GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error) {
	return c.GetRawTransactionVerboseAsync(txHash).Receive()
}

// FutureDecodeRawTransactionResult is a future promise to deliver the result
// of a DecodeRawTransactionAsync RPC invocation (or an applicable error).
type FutureDecodeRawTransactionResult chan *response

// Receive waits for the response promised by the future and returns information
// about a transaction given its serialized bytes.
func (r FutureDecodeRawTransactionResult) Receive() (*btcjson.TxRawResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a decoderawtransaction result object.
	var rawTxResult btcjson.TxRawResult
	err = json.Unmarshal(res, &rawTxResult)
	if err != nil {
		return nil, err
	}

	return &rawTxResult, nil
}

// DecodeRawTransactionAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See DecodeRawTransaction for the blocking version and more details.
func (c *Client) DecodeRawTransactionAsync(serializedTx []byte) FutureDecodeRawTransactionResult {
	txHex := hex.EncodeToString(serializedTx)
	cmd := btcjson.NewDecodeRawTransactionCmd(txHex)
	return c.sendCmd(cmd)
}

// DecodeRawTransaction returns information about a transaction given its
// serialized bytes.
func (c *Client) DecodeRawTransaction(serializedTx []byte) (*btcjson.TxRawResult, error) {
	return c.DecodeRawTransactionAsync(serializedTx).Receive()
}


//futurecreaterawtransactionresult is future pormise to deliver the result
//of a cleaterawtransactionasync ppc invaction (or an applicabe error)
type FutureCreateRawTransactionResult chan *response

// Receive waits for the response promised by the future and returns a new
// transaction spending the provided inputs and sending to the provided
// addresses.
func (r FutureCreateRawTransactionResult) Receive() (*wire.MsgTx, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var txHex string
	err = json.Unmarshal(res, &txHex)
	if err != nil {
		return nil, err
	}

	// Decode the serialized transaction hex to raw bytes.
	serializedTx, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}

	// Deserialize the transaction and return it.
	var msgTx wire.MsgTx
	// we try both the new and old encoding format
	witnessErr := msgTx.Deserialize(bytes.NewReader(serializedTx))
	if witnessErr != nil {
		legacyErr := msgTx.DeserializeNoWitness(bytes.NewReader(serializedTx))
		if legacyErr != nil {
			return nil, legacyErr
		}
	}
	return &msgTx, nil
}

// CreateRawTransactionAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See CreateRawTransaction for the blocking version and more details.
func (c *Client) CreateRawTransactionAsync(inputs []btcjson.TransactionInput,
	amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) FutureCreateRawTransactionResult {

	convertedAmts := make(map[string]float64, len(amounts))
	for addr, amount := range amounts {
		convertedAmts[addr.String()] = amount.ToBTC()
	}
	cmd := btcjson.NewCreateRawTransactionCmd(inputs, convertedAmts, lockTime)
	return c.sendCmd(cmd)
}

// CreateRawTransaction returns a new transaction spending the provided inputs
// and sending to the provided addresses.
func (c *Client) CreateRawTransaction(inputs []btcjson.TransactionInput,
	amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) (*wire.MsgTx, error) {

	return c.CreateRawTransactionAsync(inputs, amounts, lockTime).Receive()
}

// FutureSendRawTransactionResult is a future promise to deliver the result
// of a SendRawTransactionAsync RPC invocation (or an applicable error).
type FutureSendRawTransactionResult chan *response

// Receive waits for the response promised by the future and returns the result
// of submitting the encoded transaction to the server which then relays it to
// the network.
func (r FutureSendRawTransactionResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var txHashStr string
	err = json.Unmarshal(res, &txHashStr)
	if err != nil {
		return nil, err
	}

	return chainhash.NewHashFromStr(txHashStr)
}

// SendRawTransactionAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See SendRawTransaction for the blocking version and more details.
func (c *Client) SendRawTransactionAsync(tx *wire.MsgTx, allowHighFees bool) FutureSendRawTransactionResult {
	txHex := ""
	if tx != nil {
		// Serialize the transaction and convert to hex string.
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(buf); err != nil {
			return newFutureError(err)
		}
		txHex = hex.EncodeToString(buf.Bytes())
	}

	// Due to differences in the sendrawtransaction API for different
	// backends, we'll need to inspect our version and construct the
	// appropriate request.
	version, err := c.BackendVersion()
	if err != nil {
		return newFutureError(err)
	}

	var cmd *btcjson.SendRawTransactionCmd
	switch version {
	// Starting from bitcoind v0.19.0, the MaxFeeRate field should be used.
	case BitcoindPost19:
		// Using a 0 MaxFeeRate is interpreted as a maximum fee rate not
		// being enforced by bitcoind.
		var maxFeeRate int32
		if !allowHighFees {
			maxFeeRate = defaultMaxFeeRate
		}
		cmd = btcjson.NewBitcoindSendRawTransactionCmd(txHex, maxFeeRate)

	// Otherwise, use the AllowHighFees field.
	default:
		cmd = btcjson.NewSendRawTransactionCmd(txHex, &allowHighFees)
	}

	return c.sendCmd(cmd)
}

// SendRawTransaction submits the encoded transaction to the server which will
// then relay it to the network.
func (c *Client) SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error) {
	return c.SendRawTransactionAsync(tx, allowHighFees).Receive()
}

// FutureSignRawTransactionResult is a future promise to deliver the result
// of one of the SignRawTransactionAsync family of RPC invocations (or an
// applicable error).
type FutureSignRawTransactionResult chan *response

//receive waits for the message promised by the future and retuns the
//singed transaction as well as whther or not all inputs are not singed.
func(r FutureSignRawTransactionResult) Receive()(*wire.MsgTx,bool,error){
	res,err := receiveFuture(r)
	if err != nil {
		return nil,false,err
	}

	//unmarshall as a singrawtransacion result.
	var signRawTxResult btcjson.SignRawTransactionResult
	err = json.Unmarshal(res,&signRawTxResult)
	if err != nil {
		return nil,false,err
	}

	//decode the serialized trnasacion hex to raw bytes.
	serializeTx,err := hex.DecodeString(signRawTxResult.Hex)
	if err != nil {
		return nil,false,err
	}

	//deserialize the transacion and return it.
	var msgTx wire.MsgTx
	if err := msgTx.Deserialize(bytes.NewReader(serializeTx));err != nil {
		return nil, false, err
	}

	return &msgTx,singRawTxResult.complete,nil
}

//signrawtransactionasync returns an instnace of a type that can be used to
//the result of the rpc at some future time by invlking the receive function
//the returned instance.


//see singrawtransaction for the blocking version and more details.
func(c *Client)SignRawTransactionAsync(tx *wire.MsgTx)FutureSignRawTransactionResult {
	txHex := ""
	if tx != nil {
		//serialize the transaction and convert to hex string
		buf := bytes.NewBuffer(make([]byte,0,tx.SerializeSize()))
		if err := tx.Serialize(buf);err != nil {
			return newFutureError(err)

		}
		txHex = hex.EncodeToString(buf.Bytes())

	}

	cmd := btcjson.NewSignRawTransactionCmd(txHex,nil,nil)
	return c.sendCmd(cmd)
}

// SignRawTransaction signs inputs for the passed transaction and returns the
// signed transaction as well as whether or not all inputs are now signed.
//
// This function assumes the RPC server already knows the input transactions and
// private keys for the passed transaction which needs to be signed and uses the
// default signature hash type.  Use one of the SignRawTransaction# variants to
// specify that information if needed.
func (c *Client) SignRawTransaction(tx *wire.MsgTx) (*wire.MsgTx, bool, error) {
	return c.SignRawTransactionAsync(tx).Receive()
}
//signrawtransaction2async retuns an instance of a type that can be used to
//get the result of the PRC at some future time by invoking the receive
//see signRawtrnasaction2 for the blocking version and more details.
func(c *Client)SignRawTransaction2Aync(tx *wire.MsgTx,inputs []btcjson.RawTxInput) FutureSignRawTransactionResult {
	txHex := ""
	if tx != nil {
		//serialize the trnasaction and convert to hex string .
		buf := bytes.NewBuffer(make([]byte,0,tx.SerializeSize()))
		if err := tx.Serialize(buf); err != nil {
			return newFutureError(err)

		}
		txHex = hex.EncodeToString(buf.Bytes())

	}
	cmd := btcjson.NewSignRawTransactionCmd(txHex,&inputs,nil,nil)
	return c.sendCmd(cmd)
}
func (c *Client) SignRawTransaction2(tx *wire.MsgTx, inputs []btcjson.RawTxInput) (*wire.MsgTx, bool, error) {
	return c.SignRawTransaction2Async(tx, inputs).Receive()
}

// SignRawTransaction3Async returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See SignRawTransaction3 for the blocking version and more details.
func (c *Client) SignRawTransaction3Async(tx *wire.MsgTx,
	inputs []btcjson.RawTxInput,
	privKeysWIF []string) FutureSignRawTransactionResult {

	txHex := ""
	if tx != nil {
		// Serialize the transaction and convert to hex string.
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(buf); err != nil {
			return newFutureError(err)
		}
		txHex = hex.EncodeToString(buf.Bytes())
	}

	cmd := btcjson.NewSignRawTransactionCmd(txHex, &inputs, &privKeysWIF,
		nil)
	return c.sendCmd(cmd)
}

//see signrawtransaction if the PRC server alrady knows the input trnasaction
//and private keys or signRawTransaction2 if it already konws the privates keys.
func(c *Client) SignRawTransaction3(tx *wire.MsgTx, inputs []btcjson.RawTxInput,
	privKeysWIF []string) (*wire.MsgTx,bool,error){
	return c.SignRawTransaction3Async(tx,inputs,privKeysWIF).Receive()
}






















// FutureDecodeScriptResult is a future promise to deliver the result
// of a DecodeScriptAsync RPC invocation (or an applicable error).
type FutureDecodeScriptResult chan *response

// Receive waits for the response promised by the future and returns information
// about a script given its serialized bytes.
func (r FutureDecodeScriptResult) Receive() (*btcjson.DecodeScriptResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a decodescript result object.
	var decodeScriptResult btcjson.DecodeScriptResult
	err = json.Unmarshal(res, &decodeScriptResult)
	if err != nil {
		return nil, err
	}

	return &decodeScriptResult, nil
}

// DecodeScriptAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See DecodeScript for the blocking version and more details.
func (c *Client) DecodeScriptAsync(serializedScript []byte) FutureDecodeScriptResult {
	scriptHex := hex.EncodeToString(serializedScript)
	cmd := btcjson.NewDecodeScriptCmd(scriptHex)
	return c.sendCmd(cmd)
}

// DecodeScript returns information about a script given its serialized bytes.
func (c *Client) DecodeScript(serializedScript []byte) (*btcjson.DecodeScriptResult, error) {
	return c.DecodeScriptAsync(serializedScript).Receive()
}















