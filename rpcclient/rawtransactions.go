package rpcclient

import "github.com/btcsuite/btcutil"

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
