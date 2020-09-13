package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"math/big"
	"time"
)

var (
	//bignoe is 1 repeesented as a big.int it is defined here to avoid
	//the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	//oneLsh256 is 1 shifted left 256 bits .it is defined heret to
	//avoid the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

//hashtobig converts a chain.hash into a big.int that can be used to
//perform math comparisons.
func HashToBig(hash *chainhash.Hash) *big.Int {

	//a hash is in little_endian,but the big package wants the bytes in
	//big-endian.so revese them.
	buf := *hash
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
}

// CompactToBig converts a compact representation of a whole number N to an
// unsigned 32-bit number.  The representation is similar to IEEE754 floating
// point numbers.
//
// Like IEEE754 floating point, there are three basic components: the sign,
// the exponent, and the mantissa.  They are broken out as follows:
//
//	* the most significant 8 bits represent the unsigned base 256 exponent
// 	* bit 23 (the 24th bit) represents the sign bit
//	* the least significant 23 bits represent the mantissa
//
//	-------------------------------------------------
//	|   Exponent     |    Sign    |    Mantissa     |
//	-------------------------------------------------
//	| 8 bits [31-24] | 1 bit [23] | 23 bits [22-00] |
//	-------------------------------------------------
//
// The formula to calculate N is:
// 	N = (-1^sign) * mantissa * 256^(exponent-3)
//
// This compact form is only used in bitcoin to encode unsigned 256-bit numbers
// which represent difficulty targets, thus there really is not a need for a
// sign bit, but it is implemented here to stay consistent with bitcoind.

func CompactToBig(compact uint32) *big.Int {
	//extract the mantissa,sign bit,and expont.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	//since the base for exponent is 256,the exponent can be treated
	//as the number of bytes to represent the full 256-bit number .
	//so ,treat the exponent as the number of bytes and shift the mantissa.
	//right or left accourdingly.this is equivalent to :
	//N = mantissa * 256 ^(exponent -3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	//make ti negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn

}

//bigtoCompact convert a whole number N to a compact representation using
//an ussinged 32-bit number .the compact representation only provds 23 bits
//of precision ,so values larger than (2^23 -1 )only encode the most
//significant digits of the number. see compacttobig for details.
func BigToCompact(n *big.Int) uint32 {
	//no need to do any work if it is zero
	if n.Sign() == 0 {
		return 0
	}

	//since the base for the exponent is 256 the exponent can be treated
	//as the number of bytes ,so shift the number right or left
	//accordingly.this is equivalent to :
	//mantissa = mantissa / 256^(exponent - 3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		//use a copy to avoid modifying the caller original number
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact

}

// CalcWork calculates a work value from difficulty bits.  Bitcoin increases
// the difficulty for generating a block by decreasing the value which the
// generated hash must be less than.  This difficulty target is stored in each
// block header using a compact representation as described in the documentation
// for CompactToBig.  The main chain is selected by choosing the chain that has
// the most proof of work (highest difficulty).  Since a lower target difficulty
// value equates to higher actual difficulty, the work value which will be
// accumulated must be the inverse of the difficulty.  Also, in order to avoid
// potential division by zero and really small floating point numbers, the
// result adds 1 to the denominator and multiplies the numerator by 2^256.
func CalcWork(bits uint32) *big.Int {
	//return a work value of zero if the passed difficulty bits represent
	//a negative number ,note this should not happen in practice with valid
	//blocks but an invalid block could trigger it
	difficultyNum := CompactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		return big.NewInt(0)
	}

	//(1 << 256) (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, bigOne)
	return new(big.Int).Div(oneLsh256, denominator)
}

//calceasiestDiifculty calculates the easiest possible difficulty taht
//a block can have given starting difficult bits and a duraction .it is maninly used to
//verify that claimed proof of work by a block is sane as compared to a nkown good checkpoint.
func (b *BlockChain) calcEasiestDifficulty(bits uint32, duration time.Duration) uint32 {
	//convert types used in the calculations below.
	durationVal := int64(duration / time.Second)
	adjustmentFactor := big.NewInt(b.chainParams.RetargetAdjustmentFactor)

	//the test nework rules allow minimum difficulty blocks after more
	//than twice the desired amount of time needed to generate a block has
	//elapsed
	if b.chainParams.ReduceMinDifficulty {
		reductionTime := int64(b.chainParams.MinDiffReductionTime / time.Second)

		if durationVal > reductionTime {
			return b.chainParams.PowLimitBits
		}
	}

	//since easier difficult equates to higher numbers .the estiest
	//difficult for a given duration is the largest value possible given
	//the number of retargets for the duration and starting difficulty
	//multiplied by the max adjusement factor.
	newTarget := CompactToBig(bits)
	for durationVal > 0 && newTarget.Cmp(b.chainParams.PowLimit) < 0 {
		newTarget.Mul(newTarget, adjustmentFactor)
		duration -= b.maxRetargetTimespan

	}

	//limit new value to the proof of work limit.
	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
		newTarget.Set(b.chainParams.PowLimit)
	}

	return BigToCompact(newTarget)

}
