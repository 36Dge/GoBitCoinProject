package chainhash

import "crypto/sha256"

//hash b计算hash(b)并返回结果字节
func HashB(b []byte) []byte {
	hash := sha256.Sum256(b)
	return hash[:]
}

//hash计算散列 （b）并已散列形式返回结果字节.
func HashH(b []byte) Hash {
	return Hash(sha256.Sum256(b))
}

//doublehashb计算散列（hash(b)）并返回结果字节
func DoubleHashB(b []byte) []byte {
	first := sha256.Sum256(b)
	second := sha256.Sum256(first[:])
	return second[:]
}

//doublehash计算哈希（hash(b)）,并将结果字节返回为哈希
func DoubleHashH(b []byte) Hash {
	first := sha256.Sum256(b)
	return Hash(sha256.Sum256(first[:]))
}

//over
