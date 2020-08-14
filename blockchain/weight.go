package blockchain

import (
	"BtcoinProject/wire"
	"fmt"
	"github.com/btcsuite/btcutil"
)

const (
	//maxblockweight defines the maximum block wight where block
	//weight is interperted as defined in bip0141 a block,s weight
	//is calculated as the sum the of bytes in the exsiting trnasactions
	//and header plus the weight of each the weighy of a witenss byte is
	//1.as a result ,for a block to be valid,the blockweight must be less
	//than .or equal to maxblockweight.

	MaxBlockWeight = 4000000

	//maxblocksize is the maximum number of bytes within a block which
	//can be allocated to non-witness data.
	MaxBlockBaseSize = 1000000

	//maxblocksigopscost is the maximum number of signature operations
	//allowed for a block. it is calculated via a weight algorithm which
	//weight segrated witness sin ops lower than regular sig ops
	MaxBlcokSigOpsCost = 80000

	//witnesscalefactor demermines the level of "disconout" witness data
	//receiver compared to "base" data.a scale factor of 4,denotes that witmess
	//data is 1/4 as cheap as reglatar non-witness data.
	WitnessScaleFactor = 4

	//mintxoutputweight is the minimum possible wight for a transaction output
	MinTxOutputWeight = WitnessScaleFactor * wire.MinTxOutPayload

	// MaxOutputsPerBlock is the maximum number of transaction outputs there
	// can be in a block of max weight size.
	MaxOutputsPerBlock = MaxBlockWeight / MinTxOutputWeight
)

//getblockwighy computes the value of the weight metric for a given block.
//currently the weight metric is simply the sum of the block,s serialized
//size and the block,s serialized size including any witness data.
func GetBlockWeight(blk *btcutil.Block) int64 {
	msgBlock := blk.MsgBlock()

	baseSize := msgBlock.SerializeSizeStripped()
	totalSize := msgBlock.SerializeSize()

	//(baseSize * 3) + totalsize
	return int64(baseSize*(WitnessScaleFactor-1) + totalSize)
}

//gettransactionweight computes the values of the weight metric for a given
//transaction currently the weight metric is simply the sum of the trasnaction
//is serialized size without any witness data scaled  proportionally by the witnesscalfactor
//and the transaction, serialized size including any witness data.
func GetTransactionWeight(tx *btcutil.Tx) int64 {
	msgTx := tx.MsgTx()

	baseSize := msgTx.SerializeSizeStripped()
	totalSize := msgTx.SerializeSize()

	// (baseSize * 3) + totalSize
	return int64((baseSize * (WitnessScaleFactor - 1)) + totalSize)
}

//getsigopcost returns the unified sig op cost for the passed tranasction
//respecting current active soft-forks which modified sig op cost counting
//the unifed sig op cost for a transaction is computed as the sum of :
//the legacy sig op count sacled accourding to the witnesscalefactor the isg op
//count for all p2sh inputs scaled by the witnesscalefactor and finanlly
//the unscaled sig op count for any inputs spending witness prograsm.
func GetSigOpCost(tx *btcutil.Tx, isCoinBaseTx bool, utxoView *UtxoViewpoint,
	bip, segWit bool) (int, error) {

	numSigOps := countSigOps(tx) * WitnessScaleFactor
	if bip16 {
		numP2SHSigOps, err := CountP2SHSigOps(tx, isCoinBaseTx, utxoView)
		if err != nil {
			return 0, nil
		}
		numSigOps += (numP2SHSigOps * WitnessScaleFactor)
	}

	if segWit && !isCoinBaseTx {
		msgTx := tx.MsgTx()
		for txInIndex, txIn := range msgTx.TxIn {
			//ensure the referenced output is availbale and has not
			//already been spent.
			utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
			if utxo == nil || utxo.IsSpent() {
				str := fmt.Sprintf("output %v referenced from "+
					"transaction %s:%d either does not "+
					"exist or has already been spent",
					txIn.PreviousOutPoint, tx.Hash(),
					txInIndex)
				return 0, ruleError(ErrMissingTxOut, str)
			}

			witness := txIn.Witness
			sigScript := txIn.SignatureScript
			pkScript := utxo.PkScript()
			numSigOps += txscript.GetWitnessSigOpCount(sigScript, pkScript, witness)

		}
	}
	return numSigOps, nil
}

//over