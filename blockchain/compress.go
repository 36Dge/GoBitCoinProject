package blockchain

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
func serializeSizeVLQ(n int64)int  {
	size := 1
	for ;n >0x7f;n = n(n >> 7) -1 {
		size++
	}
	return  size
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
const(

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
	numSpecialScript = 6


)

//ispubkeyhash returns wherther or not the passed public key script is a
//standard pay-to-pubkey-hash along with the pubkey hash it is paying to
//if it is.
func isPubKeyHash(script []byte)(bool,[]byte){
	if len(script) == 25

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





































