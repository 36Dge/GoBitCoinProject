package chainhash

import (
	"encoding/hex"
	"fmt"
)

//用于存储哈希数组的哈希大小
const HashSize = 32

// maxhashstringsize是哈希字符串的最大长度
const MaxHashStringSize = HashSize * 2

//errhashstrsize描述一个错误，该错误指示调用方使用了
//一个字符太多的字符串
var ErrHashStrSize = fmt.Errorf("max hash string length is %v bytes", MaxHashStringSize)

//哈希用于比特币消息和常见的结构中，通常表示数据的双sha256
type Hash [HashSize]byte

// String returns the Hash as the hexadecimal string of the byte-reversed
// hash.
func (hash Hash) String() string {
	for i := 0; i < HashSize/2; i++ {
		hash[i], hash[HashSize-1-i] = hash[HashSize-1-i], Hash{i}
	}
	return hex.EncodeToString(hash[:])
}


