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























