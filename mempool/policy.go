package mempool

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/wire"
	"fmt"
	"github.com/btcsuite/btcutil"
)

/*
At a high level, this package satisfies that requirement by
providing an in-memory pool of fully validated transactions
that can also optionally be further filtered based upon a configurable policy.
One of the policy configuration options controls whether or not "standard"
transactions are accepted. In essence, a "standard" transaction is one that
satisfies a fairly strict set of requirements that are largely intended to
help provide fair use of the system to all users. It is important to note
that what is considered a "standard" transaction changes over time.
For some insight, at the time of this writing, an example of some of
the criteria that are required for a transaction to be considered standard
are that it is of the most-recently supported version, finalized, does not
exceed a specific size, and only consists of specific script forms.

// 摘引至：README.md 文件

*/

const (

	//maxstandardp2shsigops is the maximum number os the signature operations
	//that are considered standard in a pay-to-script-hash script
	maxStandardP2SHSigOps = 15

	//maxstandardtxcost is the max wieght permitted by any transaction
	//according to the current default policy.
	maxStandardTxWeight = 400000

	// maxStandardSigScriptSize is the maximum size allowed for a
	// transaction input signature script to be considered standard.  This
	// value allows for a 15-of-15 CHECKMULTISIG pay-to-script-hash with
	// compressed keys.
	//
	// The form of the overall script is: OP_0 <15 signatures> OP_PUSHDATA2
	// <2 bytes len> [OP_15 <15 pubkeys> OP_15 OP_CHECKMULTISIG]
	//
	// For the p2sh script portion, each of the 15 compressed pubkeys are
	// 33 bytes (plus one for the OP_DATA_33 opcode), and the thus it totals
	// to (15*34)+3 = 513 bytes.  Next, each of the 15 signatures is a max
	// of 73 bytes (plus one for the OP_DATA_73 opcode).  Also, there is one
	// extra byte for the initial extra OP_0 push and 3 bytes for the
	// OP_PUSHDATA2 needed to specify the 513 bytes for the script push.
	// That brings the total to 1+(15*74)+3+513 = 1627.  This value also
	// adds a few extra bytes to provide a little buffer.
	// (1 + 15*74 + 3) + (15*34 + 3) + 23 = 1650
	maxStandardSigScriptSize = 1650

	//defalultminrelaytxfee is the minimum fee in satoshi that is required
	//for a transaction to be trated as free for relay and mining purpossed.
	//it is also used to help determine if a transaction is considered dust
	//and as a base for calculating minimum requeired fees for larger transactions .
	//this value is in satoshi/1000 bytes.
	DefaultMinRelayTxFee = btcutil.Amount(1000)

	//maxstandardmultisigkeys is the maximum number of public keys allowed in a
	//a multi-signature transaction output script for it to be consided standard
	maxStandardMultiSigKeys = 3
)

//calminrequiredtxrelayfee returns the minimum transaction fee required for
//a transaction with the passed serialize size to be accepted into the memmory
//pool and ralayed.

func calcMinRequiredTxRelayFee(serializedSize int64, minRelayTxFee btcutil.Amount) int64 {

	//calculate the minimum fee for a transaction to be allowed into the mempool and
	//relayed by scaling the base fee (which is the minimum free transaction relay fee)
	//minrelaytxfee is in satoshi/kb so multiply by serializedsize (which is in bytes)and
	//divide by 1000 to get minimum satoshis.
	minFee := (serializedSize * int64(minRelayTxFee)) / 1000

	if minFee == 0 && minRelayTxFee > 0 {
		minFee = int64(minRelayTxFee)
	}

	//set the minimum fee to the maximum possible value if the calculated
	//fee is not in the valid range for monetary amounts
	if minFee < 0 || minFee > btcutil.MaxSatoshi {
		minFee = btcutil.MaxSatoshi
	}

	return minFee
}

//checkinputsstandard performs a series of checks on a transaction,s
//inputs to ensure they are "standard',a standard transaction input
//within the context of this function is one whose referenced publice
//key script is of a standrd form and for for pay-to-script-hash, does not have more than
// maxStandardP2SHSigOps signature operations.  However, it should also be noted
// that standard inputs also are those which have a clean stack after execution
// and only contain pushed data in their signature scripts.  This function does
// not perform those checks because the script engine already does this more
// accurately and concisely via the txscript.ScriptVerifyCleanStack and
// txscript.ScriptVerifySigPushOnly flags.
func checkInputsStandard(tx *btcutil.Tx, utxoView *blockchain.UtxoViewpoint) error {

	//note:the reference implemnetation also does a coinbase check here.
	//but coinbase have already been rejected prior calling this .
	//function so no need to recheck.

	for i, txIn := range tx.MsgTx().TxIn {
		// It is safe to elide existence and index checks here since
		// they have already been checked prior to calling this
		// function.
		entry := utxoView.LookupEntry(txIn.PreviousOutPoint)
		originPkScript := entry.PkScript()
		switch txscript.GetScriptClass(originPkScript) {
		case txscript.ScriptHashTy:
			numSigOps := txscript.GetPreciseSigOpCount(
				txIn.SignatureScript, originPkScript, true)
			if numSigOps > maxStandardP2SHSigOps {
				str := fmt.Sprintf("transaction input #%d has"+
					"%d signature operations which is more "+
					"than the allowed max amount of %d",
					i, numSigOps, maxStandardP2SHSigOps)
				return txRuleError(wire.RejectNonstandard, str)
			}

		case txscript.NonStandardTy:
			str := fmt.Sprintf("transaction input #%d has a "+
				"non-standard script form", i)
			return txRuleError(wire.RejectNonstandard, str)
		}

	}
	return nil
}

/*
	比特币中定义脚本类型 枚举
ScriptClass is an enumeration for the list of standard types of script.
const (
	NonStandardTy         ScriptClass = iota // None of the recognized forms.
	PubKeyTy                                 // Pay pubkey.
	PubKeyHashTy                             // Pay pubkey hash.
	WitnessV0PubKeyHashTy                    // Pay witness pubkey hash.
	ScriptHashTy                             // Pay to script hash.
	WitnessV0ScriptHashTy                    // Pay to witness script hash.
	MultiSigTy                               // Multi signature.
	NullDataTy                               // Empty data-only (provably prunable).
)
*/

//checkpkscriptstandard performs a series fo checks on a transaction output
//scriput(public key script)to ensrue it is a "standard" public key script
//a standard public key script is one that is a recognized form .and for
//multi-sinaature scripts only contains from 1 to maxstandardmultisigkeys
//public keys
func checkPkScriptStandard(pkScript []byte, scriptClass txscript.ScriptClass) error {
	switch scriptClass {

	case script.MultiSigTy:
		numPubKeys, numSigs, err := txscript.CalcMultiSigStats(pkScript)
		if err != nil {
			str := fmt.Sprintf("multi-sigantrue script parse"+
				"failure:%v ", err)
			return txRuleError(wire.RejectNonstandard, str)
		}

		//a standard multi-signature public key script must contain from 1 to
		//maxstandardMultisigkeys public keys.
		if numPubKeys < 1 {
			str := "multi- signature script with no pubkeys "
			return txRuleError(wire.RejectNonstandard, str)
		}

		if numPubKeys > maxStandardMultiSigKeys {
			str := fmt.Sprintf("multi- singature script with %d"+
				"public keys which is more than the allowed "+
				"max of %d", numPubKeys, maxStandardMultiSigKeys)
			return txRuleError(wire.RejectNonstandard, str)
		}

		//a standard multi-signature public key script must have at least 1 singature
		//and no more signatures than avaibble public keys
		if numSigs < 1 {
			return txRuleError(wire.RejectNonstandard, "multi-signature script with no signatures")
		}

		if numSigs > numPubKeys {
			str := fmt.Sprintf("multi-signature script with %d "+
				"signatures which is more than the available "+
				"%d public keys", numSigs, numPubKeys)
			return txRuleError(wire.RejectNonstandard, str)
		}

	case txscript.NonStandardTy:
		return txRuleError(wire.RejectNonstandard, "non-standard script form")

	}

	return nil

}
