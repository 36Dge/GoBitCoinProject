package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"math/big"
)

var (
	//bignoe is 1 repeesented as a big.int it is defined here to avoid
	//the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	//oneLsh256 is 1 shifted left 256 bits .it is defined heret to
	//avoid the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne,256)
)

//hashtobig converts a chain.hash into a big.int that can be used to
//perform math comparisons.
func HashToBig(hash *chainhash.Hash) *big.Int  {

	//a hash is in little_endian,but the big package wants the bytes in
	//big-endian.so revese them.
	buf := *hash
	blen := len(buf)
	for i := 0;i<blen/2;i++{
		buf[i],buf[blen - 1-i] = buf[blen-1-i],buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
}




















