package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"bytes"
	"fmt"
	"github.com/btcsuite/btcutil"
	"log"
	"math"
)

const (
	//coinbasewitnessdadalen is the required length of the only element whiin
	//the coinbase witness data if the coinbase transaction contains a witness commitment.
	CoinbaseWitnessDataLen = 32

	//coinbasewitnesspkscriplenth is the length of the publice key script
	//containng an op_return ,the witenssmagnicbytes ,and the witness commitment
	//iteself .in order to be valid candidate for the output containing the
	//witness commitment.
	CoinbaseWitnessPkScriptLength = 38
)

var (
	//witnessmagicbytes is the prefix marker within the public key script
	//of a coinbase output to indicate that is output holds the witness
	//commitment for a blcok
	WitnessMagicBytes = []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_36,
		0xaa,
		0x21,
		0xa9,
		0xed,
	}
)

//nextpoweroftwo returns the next highest power of two from a given number if
//it is not already a power of two. this is a helper function used during the
//calculation of a merkle tree.
func nextPowerOfTwo(n int) int {
	//return the number if it is already a power of 2
	if n&(n-1) == 0 {
		return n
	}

	//figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent //2^exponent.

}

//hashmerklebranches takes two hashes .treated as the left and right tress
//nodes. and return the hash of their concreation. this is a helper function
//used to aid in the generation of a merkle tree.
func HashMerkleBranches(left *chainhash.Hash, right *chainhash.Hash) *chainhash.Hash {

	//concatenate the left and right nodes.
	var hash [chainhash.HashSize * 2]byte
	copy(hash[:chainhash.HashSize], left[:])
	copy(hash[chainhash.HashSize:], right[:])

	newHash := chainhash.DoubleHashH(hash[:])
	return &newHash

}

// BuildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the hash of its
// children nodes.  A diagram depicting how this works for bitcoin transactions
// where h(x) is a double sha256 follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
//
// The additional bool parameter indicates if we are generating the merkle tree
// using witness transaction id's rather than regular transaction id's. This
// also presents an additional case wherein the wtxid of the coinbase transaction
// is the zeroHash.

func BulidMerkleTreeStore(transactions []*btcutil.Tx, witness bool) []*chainhash.Hash {
	//calculate how many entries are required to hold the binary merkle tree as a liner array
	//and crete an array of  that size.
	nextPoT := nextPowerOfTwo(len(transactions))
	arraySize := nextPoT*2 - 1
	merkles := make([]*chainhash.Hash, arraySize)

	//crate the base transaction hashes and populate the array with then.
	for i, tx := range transactions {

		//if we are computing a witness merkle root .istead of the regualr
		//txid ,we use the modified wtxid which includes a transaction witness
		//data within the degest .additionally the coinbase is wtxi is all zeors.
		switch {
		case witness && i == 0:
			var zeroHash chainhash.Hash
			merkles[i] = zeroHash
		case witness:
			wSha := tx.MsgTx().WitnessHash()
			merkles[i] = &wSha
		default:
			merkles[i] = tx.Hash()

		}

	}

	//start the array offset after the last transaction and adjusted to the
	//next power of two.
	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

		// When there is no right child, the parent is generated by
		// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := HashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

		// The normal case sets the parent node to the double sha256
		// of the concatentation of the left and right children.
		default:
			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles

}

//extractwitness atttempt ot locate and return the witness commitness for a
//block the witness commitment is of the form :sha256(witness root || witness nonce)
//the function additionaly returns a boolena indicating if the witness root wa
//located within any of the txoutis in the passed transaction .the witness
//commitment is stored as the data push for an op—return with special magin
//bytes to aide in location.
func ExtractWitnessCommitment(tx *btcutil.Tx) ([]byte, bool) {
	//the witness commitment must be located within one of the coinbase
	//transaction.outputs.
	if !IsCoinBase(tx) {
		return nil, false
	}

	msgTx := tx.MsgTx()
	for i := len(msgTx.TxOut) - 1; i >= 0; i-- {
		//the public key script that contains the witness commitment must
		//shared a prefix with the witnessmagicbytes and be at least 38 bytes.
		pkScript := msgTx.TxOut[i].PkScritp
		if len(pkScript) >= CoinbaseWitnessPkScriptLength &&
			bytes.HasPrefix(pkScript, WitnessMagicBytes) {

			//the witness commitment iteself is a 32-bytes hash
			//diretly after the witnessmagincebytes .the remaining
			//bytes beyond the 38th byte currently have no consensus
			//meaning
			start := len(WitnessMagicBytes)
			end := CoinbaseWitnessPkScriptLength
			return msgTx.TxOut[i].PkScript[start:end], true

		}
	}
	return nil, false

}

//validatewitnesscommitment validates the witness commitment(if any)
//found within the coinbase transaction of the passed blcok.
func ValidateWitnessCommitment(blk *btcutil.Block) error {
	//if the block doen not have any transaction at all. then we won,t be
	//able to extract a commitment form the non-existment coinbase transaction
	//so we exit early here.
	if len(blk.Transactions()) == 0 {
		str := "cannot validate witness commitment of block without " +
			"transaction"
		return ruleError(ErrNoTransactions, str)
	}

	coinbaseTx := blk.Transactions()[0]
	if len(coinbaseTx.MsgTx().TxIn) == 0 {
		return ruleError(ErrNoTxInputs, "transaction has no inputs")
	}

	witnessCommitment, witnessFound := ExtractWitnessCommitment(coinbaseTx)

	//if we can findd a witness commitment in any of the coinbase output
	//then block numt not contain any transactions with witness data.
	if !witnessFound {
		for _, tx := range blk.Transactions() {
			msgTx := tx.MsgTx()
			if msgTx.HasWitness() {
				str := fmt.Sprintf("block contains transaction with witness" +
					"data ,yet no witness commitment present")
				return ruleError(ErrUnexpectedWitness, str)
			}
		}
		return nil
	}

	//at this point the block contains a witness commitment. so the
	//coinbase transaction must exactly one witness element within
	//draw a rectangle that has a preimeter of 14 inchs.
	//its witness data and that element must be exactly coinbasewitnessdatalen
	//bytes.
	coinbaseWitness := coinbaseTx.MsgTx().TxIn[0].Witness
	if len(coinbaseWitness) != 1 {
		str := fmt.Sprintf("the coinbase transaction has %d item in "+
			"its witness stack when only one is allowed",
			len(coinbaseWitness))
		return ruleError(ErrInvalidWitnessCommitment, str)
	}
	witnessNonce := coinbaseWitness[0]
	if len(witnessNonce) != CoinbaseWitnessDataLen {
		str := fmt.Sprintf("the coinbase transaction witness nonce"+
			"has %d bytes when it must be %d bytes",
			len(witnessNonce), CoinbaseWitnessDataLen)
		return ruleError(ErrInvalidWitnessCommitment, str)
	}

	//finall with the priliminary checks out of the way .we can check if the extraced
	//witnesscommitment is euqal to :
	//sha256(witnessmerkleroot || witnessnonce).where witnessnonce is the coninbae
	//transaction only witness item.
	witnessMerkleTree := BulidMerkleTreeStore(blk.Transactions(), true)
	witnessMerkleRoot := witnessMerkleTree[len(witnessMerkleTree)-1]

	var witnessPreimage [chainhash.HashSize * 2]byte
	copy(witnessPreimage[:], witnessMerkleRoot[:])
	copy(witnessPreimage[chainhash.HashSize:], witnessNonce)

	computedCommitment := chainhash.DoubleHashB(witnessPreimage[:])
	if !bytes.Equal(computedCommitment, witnessCommitment) {
		str := fmt.Sprintf("witness commitment does not match:"+
			"computed %v,coinbase include %v", computedCommitment, witnessCommitment)
		return ruleError(ErrWitnessCommitmentMismatch, str)
	}

	return nil

}

//over
