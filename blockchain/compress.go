package blockchain

import (
	"BtcoinProject/txscript"
	"golang.org/x/text/language"
)

// -----------------------------------------------------------------------------
// A variable length quantity (VLQ) is an encoding that uses an arbitrary number
// of binary octets to represent an arbitrarily large integer.  The scheme
// employs a most significant byte (MSB) base-128 encoding where the high bit in
// each byte indicates whether or not the byte is the final one.  In addition,
// to ensure there are no redundant encodings, an offset is subtracted every
// time a group of 7 bits is shifted out.  Therefore each integer can be
// represented in exactly one way, and each representation stands for exactly
// one integer.
//
// Another nice property of this encoding is that it provides a compact
// representation of values that are typically used to indicate sizes.  For
// example, the values 0 - 127 are represented with a single byte, 128 - 16511
// with two bytes, and 16512 - 2113663 with three bytes.
//
// While the encoding allows arbitrarily large integers, it is artificially
// limited in this code to an unsigned 64-bit integer for efficiency purposes.
//
// Example encodings:
//           0 -> [0x00]
//         127 -> [0x7f]                 * Max 1-byte value
//         128 -> [0x80 0x00]
//         129 -> [0x80 0x01]
//         255 -> [0x80 0x7f]
//         256 -> [0x81 0x00]
//       16511 -> [0xff 0x7f]            * Max 2-byte value
//       16512 -> [0x80 0x80 0x00]
//       32895 -> [0x80 0xff 0x7f]
//     2113663 -> [0xff 0xff 0x7f]       * Max 3-byte value
//   270549119 -> [0xff 0xff 0xff 0x7f]  * Max 4-byte value
//      2^64-1 -> [0x80 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0xfe 0x7f]
//
// References:
//   https://en.wikipedia.org/wiki/Variable-length_quantity
//   http://www.codecodex.com/wiki/Variable-Length_Integers
// -----------------------------------------------------------------------------

//serializedsize returns the number of bytes it would to serialize the
//passed number as a variable-length quantitiy according to the format desscribed
//above.
func serializeSizeVLQ(n uint64) int {
	size := 1
	for ; n > 0x7f; n = n(n>>7) - 1 {
		size++
	}
	return size
}

// putVLQ serializes the provided number to a variable-length quantity according
// to the format described above and returns the number of bytes of the encoded
// value.  The result is placed directly into the passed byte slice which must
// be at least large enough to handle the number of bytes returned by the
// serializeSizeVLQ function or it will panic.
func putVLQ(target []byte, n uint64) int {
	offset := 0
	for ; ; offset++ {
		// The high bit is set when another byte follows.
		highBitMask := byte(0x80)
		if offset == 0 {
			highBitMask = 0x00
		}

		target[offset] = byte(n&0x7f) | highBitMask
		if n <= 0x7f {
			break
		}
		n = (n >> 7) - 1
	}

	// Reverse the bytes so it is MSB-encoded.
	for i, j := 0, offset; i < j; i, j = i+1, j-1 {
		target[i], target[j] = target[j], target[i]
	}

	return offset + 1
}

// deserializeVLQ deserializes the provided variable-length quantity according
// to the format described above.  It also returns the number of bytes
// deserialized.
func deserializeVLQ(serialized []byte) (uint64, int) {
	var n uint64
	var size int
	for _, val := range serialized {
		size++
		n = (n << 7) | uint64(val&0x7f)
		if val&0x80 != 0x80 {
			break
		}
		n++
	}

	return n, size
}

//in order to reduce the size of stored scriptes a domin specific
//compression algorithm is used which recognize standared scriptes and
//stores them using less bytes than the original script.the compression
//algorithm used here was obtained from bitcoin core.so all credits for
//the algorithm go to it.
//the general serialized format is:
//<script size or type> <scripte data>
//field 	type 	size
//script size or type vlq variable
//script data []byte variable

//the specific serialised format for each recoginized standard scripte is:
// - Pay-to-pubkey-hash: (21 bytes) - <0><20-byte pubkey hash>
// - Pay-to-script-hash: (21 bytes) - <1><20-byte script hash>
// - Pay-to-pubkey**:    (33 bytes) - <2, 3, 4, or 5><32-byte pubkey X value>
//   2, 3 = compressed pubkey with bit 0 specifying the y coordinate to use
//   4, 5 = uncompressed pubkey with bit 0 specifying the y coordinate to use
//   ** Only valid public keys starting with 0x02, 0x03, and 0x04 are supported.
//
// Any scripts which are not recognized as one of the aforementioned standard
// scripts are encoded using the general serialized format and encode the script
// size as the sum of the actual size of the script and the number of special
// cases.

// the following constants specifiy the constants used to identify a
//special script type in the domain-specific compressed script encoding
//note:this section sepcifically does not use iota since these values are
//serialized and must be stable for long-term storage.
const (

	//cstpaytopubkeyhash identify a compressed pay to pubkey hahs script.
	cstPayToPubKeyHash = 0

	//cstpaytoscripthash identify a compressed pay-to-script hash script.
	cstPayToScritpHash = 1

	//cstpaytopubkeycomp2 identifies a compressed pat-to-pubkey script to a
	//compressed pubkey ,bit 0 spcecifies which y-coordinate to use to reconstruct the
	//full uncompressed pubkey.
	cstPayToPubKeyComp2 = 2

	// cstPayToPubKeyComp3 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyComp3 = 3

	// cstPayToPubKeyUncomp4 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp4 = 4

	// cstPayToPubKeyUncomp5 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp5 = 5

	//numspecialscripts is the number of special scripts recognized by the
	//domain-specific script compression algorithm .
	numSpecialScripts = 6
)

//ispubkeyhash returns wherther or not the passed public key script is a
//standard pay-to-pubkey-hash along with the pubkey hash it is paying to
//if it is.
func isPubKeyHash(script []byte) (bool, []byte) {
	if len(script) == 25 && script[0] == txscript.OP_DUP &&
		script[1] == txscript.OP_HASH160 &&
		script[2] == txscript.OP_DATA_20 &&
		script[23] == txscript.OP_EQUALVERIFY &&
		script[24] == txscript.OP_CHECKSIG {
		return true, script[3:23]
	}

	return false, nil

}

//isscriptHash returns whether or not the passed public key script is a standard pay
//to -scirpt along with the scirpt hash it is paying to
//if it is .
func isScriptHash(script []byte) (bool, []byte) {
	if len(script) == 23 && script[0] == txscript.OP_HASH160 &&
		script[1] == txscript.OP_DATA_20 &&
		script[22] == txscript.OP_EQUAL {
		return true, script[2:22]
	}
	return false, nil
}

//isPubkey returns whether or not the passed public key script is a standard
//payt to pubkey script that pays to a valid compressed or uncompressed public
//key along with the serialized pubkey it is paying to if it is.

//note:this fucntion ensures the public key is actulaaly valid since the compression
//algorithm require valid pubkeys.it does not suport hybrid pubkeys this means
//that even if the script has the corrent form for a pay_to_public script .this
//function will only returns true when it is paying to a valid compressed or unpressed public.

func isPubKey(script []byte) (bool, []byte) {
	//payt -to -compressed pubkey script.
	if len(script) == 35 && script[0] == txscript.OP_DATA_33 &&
		script[34] == txscript.OP_CHECKSIG && (script[1] == 0x02 ||
		script[1] == 0x03) {

		//ensure the public key is valid.
		serializeSizePubKey := script[1:34]
		_, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
		if err == nil {
			return true, serializeSizePubKey
		}
	}

	//pay to uncompressed-pubkey script
	if len(script) == 67 && script[0] == txscript.OP_DATA_65 &&
		script[66] == txscript.OP_CHECKSIG && script[1] == 0x04 {

		//ensure the public key is valid
		serializePubKey := script[1:66]
		_, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
		if err == nil {
			return true, serializePubKey
		}
	}
	return false, nil
}

//comperssedscriptsize returns the number of bytes the passed script would take
//when encoded with the domain specific compression algorithm described above.
func compressedScriptSize(pkScript []byte) int {

	//paytopubkeyhash script
	if valid, _ := isPubKeyHash(pkScript); valid {
		return 21

	}

	//paytoscripthash script.
	if valid, _ := isScriptHash(pkScript); valid {
		return 21
	}

	//paytopubkey(compressed and uncompressed)script
	if valid, _ := isPubKeyHash(pkScript); valid {
		return 33
	}

	//when none of the above special cases apply.encode the script as is
	//preceded by the sum of its size and the number of special cases encoded
	//as a varibale length quantity.
	return serializeSizeVLQ(uint64(len(pkScript)+numSpecialScripts)) +
		len(pkScript)

}

//decodecompressedscriptsize treats the passed serialized bytes as a compressed
//script possbily followed by other data.and returns the number of bytes it
//occupies taking into account be specail encoding of the script size by the
//domain specific compression algorithm described above.
func decodeCompressedScriptSize(serialized []byte) int {

	scriptSize, bytesRead := deserializeVLQ(serialized)
	if bytesRead == 0 {
		return 0
	}

	switch scriptSize {
	case cstPayToPubKeyHash:
		return 21
	case cstPayToScritpHash:
		return 21

	case cstPayToPubKeyComp2, cstPayToPubKeyComp3, cstPayToPubKeyUncomp4,
		cstPayToPubKeyUncomp5:
		return 33
	}

	scriptSize -= numSpecialScripts
	scriptSize += uint64(bytesRead)

	return int(scriptSize)

}

//putcompressscript compress the passed script accouding to the domian
//specific compression algorithm descrirbed above directly into the passd
//target byte slice .the target byte slice must be at least large enoubgh yo
//handle the number of bytes returned by the compressedScriptSize function oor
//it will panic
func putCompressedScript(target, pkScript []byte) int {
	//paytopubkeyhash script
	if valid, hash := isPubKeyHash(pkScript); valid {
		target[0] = cstPayToScritpHash
		copy(target[1:21], hash)
		return 21

	}

	//pay-to-script-hash script
	if valid, hash := isScriptHash(pkScript); valid {
		target[0] = cstPayToScritpHash
		copy(target[1:21], hash)
		return 21

	}

	//pay-to-pubkey(compressed or uncompressed)script.
	if valid, serializedPubKey := isPubKeyHash(pkScript); valid {
		pubKeyFormat := serializedPubKey[0]
		switch pubKeyFormat {
		case 0x02, 0x03:
			target[0] = pubKeyFormat
			copy(target[1:33], serializedPubKey[1:33])
			return 33
		case 0x04:
			//encode the oddness of the serialized pubkey into the
			//compressed script type.
			target[0] = pubKeyFormat | (serializedPubKey[64] & 0x01)
			copy(target[1:33], serializedPubKey[1:33])
			return 33
		}
	}

	//when none of the above special cases apply,encode the unmodified
	//script preceded by the sum of its size and the number of special
	//cases encoded as a variable length quantity.
	encodedSize := uint64(len(pkScript) + numSpecialScripts)
	vlqsizeLen := putVLQ(target, encodedSize)
	copy(target[vlqsizeLen:], pkScript)
	return vlqsizeLen + len(pkScript)

}

//decompressscript returns the original script obtained by decompressinog the
//passed compressed socript according to the domain specific compression
//algorithm described above.
//Note:the script parameter must be already have been proven to be long enought
//to contain the number of bytes returned by decodeCompressedScriptsize or it
//will panic .this is acceptalbe since it is only an internal function.
func decompressScript(compressedPkScript []byte) []byte {
	//in practise this function will not be called with a zero-length or
	//nil script since the nil script encoding includes th length.howenver
	//the code below assumes the length exists,so just return nil how if
	//the function ever up being called with a nil script in the future .
	if len(compressedPkScript) == 0 {
		return nil
	}

	//decode the script size and examine it for the special cases.
	encodeScriptSize, bytesRead := deserializeVLQ(compressedPkScript)
	switch encodeScriptSize {

	//pay-to-pubkey-hash-script the resulting script is :
	// <OP_DUP><OP_HASH160><20 byte hash><OP_EQUALVERIFY><OP_CHECKSIG>
	case cstPayToPubKeyHash:
		pkScript := make([]byte, 25)
		pkScript[0] = txscript.OP_DUP
		pkScript[1] = txscript.OP_HASH160
		pkScript[2] = txscript.OP_DATA_20
		copy(pkScript[3:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[23] = txscript.OP_EQUALVERIFY
		pkScript[24] = txscript.OP_CHECKSIG
		return pkScript

	//pay-to-script-hash script .the resulting script is :
	// <OP_DUP><OP_HASH160><20 byte hash><OP_EQUALVERIFY><OP_CHECKSIG>

	case cstPayToScritpHash:
		pkScript := make([]byte, 23)
		pkScript[0] = txscript.OP_HASH160
		pkScript[1] = txscript.OP_DATA_20
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[22] = txscript.OP_EQUAL
		return pkScript

		// Pay-to-compressed-pubkey script.  The resulting script is:
		// <OP_DATA_33><33 byte compressed pubkey><OP_CHECKSIG>
	case cstPayToPubKeyComp2, cstPayToPubKeyComp3:
		pkScript := make([]byte, 35)
		pkScript[0] = txscript.OP_DATA_33
		pkScript[1] = byte(encodedScriptSize)
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+32])
		pkScript[34] = txscript.OP_CHECKSIG
		return pkScript

	case cstPayToPubKeyUncomp4, cstPayToPubKeyUncomp5:
		//change the leading byte to the approriate compressed pubkey
		//idenfitier so it can be decoded as a idenfitier so it can be
		//decodes as a compressed pubkey.this really shoulld never fail since
		//the encoding ensures it it valid before compressing to this byte.
		compressedKey := make([]byte, 33)
		compressedKey[0] = byte(encodedScriptSize - 2)
		copy(compressedKey[1:], compressedPkScript[1:])
		key, err := btcec.ParsePubKey(compressedKey, btcec.S256())
		if err != nil {
			return nil
		}

		pkScript := make([]byte, 67)
		pkScript[0] = txscript.OP_DATA_65
		copy(pkScript[1:], key.SerializeUncompressed())
		pkScript[66] = txscript.OP_CHECKSIG
		return pkScript

	}
	//when none of the special cases apply.the script was encoded using
	//the general format .so reduce the script size by the number of
	//special cases and return the unmodified script.
	scriptSize := int(encodedScriptSize - numSpecialScripts)
	pkScript := make([]byte, scriptSize)
	copy(pkScript, compressedPkScript[bytesRead:bytesRead+scriptSize])
	return pkScript

}
// -----------------------------------------------------------------------------
// Compressed transaction outputs consist of an amount and a public key script
// both compressed using the domain specific compression algorithms previously
// described.
//
// The serialized format is:
//
//   <compressed amount><compressed script>
//
//   Field                 Type     Size
//     compressed amount   VLQ      variable
//     compressed script   []byte   variable
// -----------------------------------------------------------------------------

// compressedTxOutSize returns the number of bytes the passed transaction output
// fields would take when encoded with the format described above.

//-------

// -----------------------------------------------------------------------------
// In order to reduce the size of stored amounts, a domain specific compression
// algorithm is used which relies on there typically being a lot of zeroes at
// end of the amounts.  The compression algorithm used here was obtained from
// Bitcoin Core, so all credits for the algorithm go to it.
//
// While this is simply exchanging one uint64 for another, the resulting value
// for typical amounts has a much smaller magnitude which results in fewer bytes
// when encoded as variable length quantity.  For example, consider the amount
// of 0.1 BTC which is 10000000 satoshi.  Encoding 10000000 as a VLQ would take
// 4 bytes while encoding the compressed value of 8 as a VLQ only takes 1 byte.
//
// Essentially the compression is achieved by splitting the value into an
// exponent in the range [0-9] and a digit in the range [1-9], when possible,
// and encoding them in a way that can be decoded.  More specifically, the
// encoding is as follows:
// - 0 is 0
// - Find the exponent, e, as the largest power of 10 that evenly divides the
//   value up to a maximum of 9
// - When e < 9, the final digit can't be 0 so store it as d and remove it by
//   dividing the value by 10 (call the result n).  The encoded value is thus:
//   1 + 10*(9*n + d-1) + e
// - When e==9, the only thing known is the amount is not 0.  The encoded value
//   is thus:
//   1 + 10*(n-1) + e   ==   10 + 10*(n-1)
//
// Example encodings:
// (The numbers in parenthesis are the number of bytes when serialized as a VLQ)
//            0 (1) -> 0        (1)           *  0.00000000 BTC
//         1000 (2) -> 4        (1)           *  0.00001000 BTC
//        10000 (2) -> 5        (1)           *  0.00010000 BTC
//     12345678 (4) -> 111111101(4)           *  0.12345678 BTC
//     50000000 (4) -> 47       (1)           *  0.50000000 BTC
//    100000000 (4) -> 9        (1)           *  1.00000000 BTC
//    500000000 (5) -> 49       (1)           *  5.00000000 BTC
//   1000000000 (5) -> 10       (1)           * 10.00000000 BTC
// -----------------------------------------------------------------------------

// compressTxOutAmount compresses the passed amount according to the domain
// specific compression algorithm described above.

func compressTxOutAmount(amount uint64) uint64{
	//no need to do any work if it is zero.
	if amount == 0 {
		return 0
	}

	//find the largest power of 10 (max of 9) that envenly divides the
	//value
	exponent := uint64(0)
	for amount % 10 == 0 && exponent < 9 {
		amount /= 10
		exponent++
	}

	//the compressed result for exponents less than 9 is:
	if exponent < 9 {
		lastDigit := amount % 10
		amount /= 10
		return 1 + 10*(9*amount + lastDigit-1) + exponent
	}

	//the compressed result for an expont of 9 is :
	return 10 + 10*(amount - 1)


}


















