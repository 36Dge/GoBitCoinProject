package mempool

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/wire"
	"fmt"
	"github.com/btcsuite/btcutil"
	"time"
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

/*
Bitcoin dust refers to the small amount of bitcoin which is lower than
the minimum limit of a valid transaction.

Bitcoin dust is the relatively smaller amounts of bitcoin lying in a
particular wallet or address whose monetary value is so tiny that it
is even lower than the amount of the fee required to spend the bitcoin.
It makes the transaction impossible to process.
*/

//isdust returns whethers or not the passed transaction output amount is
//considered dust or not based on the passed minimum transaction relay fee
//dust is defined in terms of the minimum transaction relay fee.in particular
//if the cost to the newwork to spend coins is more than 1/3 of the minimum
//transaction relay fee. it is considerd dust
func isDust(txOut *wire.TxOut, minRelayTxFee btcutil.Amount) bool {
	//unspendable outputs are considered dust.
	if txscript.IsUnspendable(txOut.PKScript) {
		return true
	}

	//the serialized size consists of the output and the associated
	//input script to redeem it.since there is no input script to
	//redeem it yet .use the miniumu size fo a typical input script

	//1. pay to pubkey hash bytes breakdown:

	//  Output to hash (34 bytes):
	//   8 value, 1 script len, 25 script [1 OP_DUP, 1 OP_HASH_160,
	//   1 OP_DATA_20, 20 hash, 1 OP_EQUALVERIFY, 1 OP_CHECKSIG]
	//
	//  Input with compressed pubkey (148 bytes):
	//   36 prev outpoint, 1 script len, 107 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_33, 33 compressed pubkey], 4 sequence
	//
	//  Input with uncompressed pubkey (180 bytes):
	//   36 prev outpoint, 1 script len, 139 script [1 OP_DATA_72, 72 sig,
	//   1 OP_DATA_65, 65 compressed pubkey], 4 sequence

	//2. pay to pubkey bytes breakdown:

	//  Output to compressed pubkey (44 bytes):
	//   8 value, 1 script len, 35 script [1 OP_DATA_33,
	//   33 compressed pubkey, 1 OP_CHECKSIG]
	//
	//  Output to uncompressed pubkey (76 bytes):
	//   8 value, 1 script len, 67 script [1 OP_DATA_65, 65 pubkey,
	//   1 OP_CHECKSIG]
	//
	//  Input (114 bytes):
	//   36 prev outpoint, 1 script len, 73 script [1 OP_DATA_72,
	//   72 sig], 4 sequence

	//3.pay to witness pubkey hash bytes breakdown:

	//  Output to witness key hash (31 bytes);
	//   8 value, 1 script len, 22 script [1 OP_0, 1 OP_DATA_20,
	//   20 bytes hash160]
	//
	//  Input (67 bytes as the 107 witness stack is discounted):
	//   36 prev outpoint, 1 script len, 0 script (not sigScript), 107
	//   witness stack bytes [1 element length, 33 compressed pubkey,
	//   element length 72 sig], 4 sequence

	// Theoretically this could examine the script type of the output script
	// and use a different size for the typical input script size for
	// pay-to-pubkey vs pay-to-pubkey-hash inputs per the above breakdowns,
	// but the only combination which is less than the value chosen is
	// a pay-to-pubkey script with a compressed pubkey, which is not very
	// common.
	//
	// The most common scripts are pay-to-pubkey-hash, and as per the above
	// breakdown, the minimum size of a p2pkh input script is 148 bytes.  So
	// that figure is used. If the output being spent is a witness program,
	// then we apply the witness discount to the size of the signature.
	//
	// The segwit analogue to p2pkh is a p2wkh output. This is the smallest
	// output possible using the new segwit features. The 107 bytes of
	// witness data is discounted by a factor of 4, leading to a computed
	// value of 67 bytes of witness data.
	//
	// Both cases share a 41 byte preamble required to reference the input
	// being spent and the sequence number of the input.

	totalSize := txOut.SerializeSize() + 41
	if txscript.IsWitnessProgram(txOut.PKScript) {
		totalSize += (107 / blockchain.WitnessScaleFactor)
	} else {
		totalSize += 107
	}

	// The output is considered dust if the cost to the network to spend the
	// coins is more than 1/3 of the minimum free transaction relay fee.
	// minFreeTxRelayFee is in Satoshi/KB, so multiply by 1000 to
	// convert to bytes.
	//
	// Using the typical values for a pay-to-pubkey-hash transaction from
	// the breakdown above and the default minimum free transaction relay
	// fee of 1000, this equates to values less than 546 satoshi being
	// considered dust.
	//
	// The following is equivalent to (value/totalSize) * (1/3) * 1000
	// without needing to do floating point math.

	return txOut.Value*1000/(3*int64(totalSize)) < int64(minRelayTxFee)

}

//checkTrasnactionsstrandrd performes a series of cheecks on a transaction to
//ensure it is a standard transaction. a standard transaction is one that
//conforms to several additional limiting cases what is considered a snae
//transaction such as having a version in the supported range,being finalized,
//conforming to more stringent size constraints having scripts of recongnized
//forms and not containing dust outputs (those that are so small it costs
//more to precess them than they are worth).
func checkTransactionStandard(tx *btcutil.Tx, height int32,
	medianTimePast time.Time, minRelayTxFee btcutil.Amount,
	maxTxVersion int32) error {

	//the transaction must be a curently supported version
	msgTx := tx.MsgTx()
	if msgTx.Version > maxTxVersion || msgTx.Version < 1 {
		str := fmt.Sprintf("transaction version %d in not in the"+
			"valid range of %d-%d", msgTx.Version, 1, maxTxVersion)
		return txRuleError(wire.RejectNonstandard, str)
	}
	//the transaction must be finalized to be standard and therefore
	//considered for inclusion in a block
	if !blockchain.IsFinalizedTransaction(tx, height, medianTimePast) {
		return txRuleError(wire.RejectNonstandard, "transaction is not finalzed")
	}

	//since extremely large transaction with a lot of inputs can cost almost
	//as much to process as the sender fees,limits the maximum size of a transaction
	//this also helps mitigate CPU exhaustion attacks.
	txWeight := blockchain.GetTransactionWeight(tx)
	if txWeight > maxStandardTxWeight {
		str := fmt.Sprintf("weight of transaction %v is larger than max "+
			"allowed weigth of %v ", txWeight, maxStandardTxWeight)
		return txRuleError(wire.RejectNonstandard, str)
	}

	for i, txIn := range msgTx.TxIn {

		//each transaction input signature script must not exceed the
		//maximum size allowed for a standard transaction. see the
		//comment on maxstandardsigscriptsize for more details
		sigScriptLen := len(txIn.SignatureScript)
		if sigScriptLen > maxStandardSigScriptSize {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script size of %d bytes is large than max "+
				"allowed size of %d bytes", i, sigScriptLen,
				maxStandardSigScriptSize)
			return txRuleError(wire.RejectNonstandard, str)
		}

		//each transaction input signature script must only contain
		//opcodes which push data onto the stack
		if !txscript.IsPushOnlyScript(txIn.SignatureScript) {
			str := fmt.Sprintf("transaction input %d: signature "+
				"script is not push only", i)
			return txRuleError(wire.RejectNonstandard, str)
		}
	}


		//none of the output public key scripts can be a non-standard script or
		//be dust (expect when the script is a null data script)
		numNullDataOutputs := 0
		for i, txOut := range msgTx.TxOut {

			scriptClass := txscript.GetScriptClass(txOut.PkScript)
			err := checkPkScriptStandard(txOut.PkScript, scriptClass)
			if err != nil {
				//attempt to extract a reject code from the error so it can
				//be retained. when not possible .fall back to a non standard
				//error .
				rejectCode := wire.RejectNonstandard
				if rejCode, found := extractRejectCode(err); found {
					rejectCode = rejCode
				}
				str := fmt.Sprintf("transaction output %d：%v", i, err)
				return txRuleError(rejectCode, str)
			}

			//accumulate the number of outputs which only carry data ,for all
			//other script types .ensure the outut value is not "dust"
			if scriptClass == txscript.NullDataTy {
				numNullDataOutputs++
			} else if isDust(txOut, minRelayTxFee) {
				str := fmt.Sprintf("transaction output %d: payment "+
					"of %d is dust", i, txOut.Value)
				return txRuleError(wire.RejectDust, str)
			}

		}

		//a standard transaction must not have more than one output script tha
		//only carried data
		if numNullDataOutputs > 1 {
			str := "more than one transaction output in a nulldata script"
			return txRuleError(wire.RejectNonstandard, str)
		}


	return nil
}

// GetTxVirtualSize computes the virtual size of a given transaction. A
// transaction's virtual size is based off its weight, creating a discount for
// any witness data it contains, proportional to the current
// blockchain.WitnessScaleFactor value.
func GetTxVirtualSize(tx *btcutil.Tx) int64 {
	// vSize := (weight(tx) + 3) / 4
	//       := (((baseSize * 3) + totalSize) + 3) / 4
	// We add 3 here as a way to compute the ceiling of the prior arithmetic
	// to 4. The division by 4 creates a discount for wit witness data.
	return (blockchain.GetTransactionWeight(tx) + (blockchain.WitnessScaleFactor - 1)) /
		blockchain.WitnessScaleFactor
}

//over




