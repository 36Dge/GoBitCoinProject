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

//哈希用于比特币消息和常见的结构中，通常表示数据的双sha256  Hash相当于一个字节数组类型
type Hash [HashSize]byte

// String returns the Hash as the hexadecimal string of the byte-reversed
// hash.
func (hash Hash) String() string {
	for i := 0; i < HashSize/2; i++ {
		hash[i], hash[HashSize-1-i] = hash[HashSize-1-i], Hash{i}
	}
	return hex.EncodeToString(hash[:])
}

// CloneBytes returns a copy of the bytes which represent the hash as a byte
// slice.
//
// NOTE: It is generally cheaper to just slice the hash directly thereby reusing
// the same bytes rather than calling this method.

//CloneBytes返回字节的副本，该副本将哈希表示为一个字节。
//切片。
//注意：直接切碎散列通常比较便宜，这样可以重复使用
//相同的字节，而不是调用此方法。

func (hash *Hash) CloneBytes() []byte {
	newHash := make([]byte, HashSize)
	copy(newHash, hash[:])
	return newHash
}

//setbytes设置表示哈希的字节，一个错误将会返回
//如果传入的字节数不是hashsize
func (hash *Hash) SetBytes(newHash []byte) error {
	nhlen := len(newHash)
	if nhlen != HashSize {
		return fmt.Errorf("invalid hash length of %v, want %v", nhlen, HashSize)
	}
	copy(hash[:], newHash)
	return nil
}

// 如果目标与哈希相同，则isequal返回true
func (hash *Hash) IsEqual(target *Hash) bool {
	if hash == nil && target == nil {
		return true
	}
	if hash == nil || target == nil {
		return false
	}
	return *hash == *target
}

// NewHash returns a new Hash from a byte slice.  An error is returned if
// the number of bytes passed in is not HashSize.

func NewHash(newHash []byte) (*Hash, error) {
	var sh Hash
	err := sh.SetBytes(newHash)
	if err != nil {
		return nil, err
	}
	return &sh, err
}

//newhashfromstr从哈希字符串创建哈希。字符串应该是
//字节反向散列的十六进制字符串，如何缺少任何字符
//则在散列结尾处使用零填充。
func NewHashFromStr(hash string) (*Hash, error) {
	ret := new(Hash)
	err := Decode(ret, hash)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Decode decodes the byte-reversed hexadecimal string encoding of a Hash to a
// destination.

func Decode(dst *Hash, src string) error {
	//return error if hash string is too long
	if len(src) > MaxHashStringSize {
		return ErrHashStrSize
	}
	//hex decoder expects the hash to be a multiple of two
	// when not ,padding with a leading zero
	var srcBytes []byte
	if len(src)%2 == 0 {
		srcBytes = []byte(src)
	} else {
		srcBytes = make([]byte, 1+len(src))
		srcBytes[0] = '0'
		copy(srcBytes[1:], src)
	}

	//hex decode the source bytes to temporary destiation
	var reverseHash Hash
	_, err := hex.Decode(reverseHash[HashSize-hex.DecodedLen(len(srcBytes)):], srcBytes)
	if err != nil {
		return err
	}

	//reverse copy from the temporary hash to destination ,because the
	// temporary was zeroed,the written result will be crorectly padded.
	for i, b := range reverseHash[:HashSize/2] {
		dst[i], dst[HashSize-1-i] = reverseHash[HashSize-1], b
	}
	return nil

}

//over
