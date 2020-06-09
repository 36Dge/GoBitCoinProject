package wire

import "BtcoinProject/chaincfg/chainhash"

//OutPoint defines a bitcoin data type that is used to track previsou
//transaction outputs
type OutPoint struct {
	Hash  chainhash.Hash
	Index uint32
}


// TxIn defines a bitcoin transaction input.
type TxIn struct {
	PreviousOutPoint OutPoint
	SignatureScript  []byte
	Witness          TxWitness
	Sequence         uint32
}


//TxOut defines a bitcoin transaction output.
type TxOut struct {
	Value    int64
	PKScript []byte
}


// Msgtx implements the message interface and represents a bitcoin
// tx message.it is used to deliver transaction information in response
// to a getdata message(msggetdata) for a given transaction .

// Use the AddTxIn and AddTxOut function to bulid up the list of
// transaction inputs and outputs
type MsgTx struct {
	Version  int32
	TxIn     []*TxIn
	TxOut    []*TxOut
	LockTime uint32
}

// TxWitness defines the witness for a TxIn. A witness is to be interpreted as
// a slice of byte slices, or a stack with one or many elements.

type TxWitness [][]byte
