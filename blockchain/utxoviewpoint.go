package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"github.com/btcsuite/btcutil"
)

// txoFlags is a bitmask defining additional information and state for a
// transaction output in a utxo view.
type txoFlags uint8

const (
	// tfCoinBase indicates that a txout was contained in a coinbase tx.
	tfCoinBase txoFlags = 1 << iota

	// tfSpent indicates that a txout is spent.
	tfSpent

	// tfModified indicates that a txout has been modified since it was
	// loaded.
	tfModified
)

// UtxoEntry houses details about an individual transaction output in a utxo
// view such as whether or not it was contained in a coinbase tx, the height of
// the block that contains the tx, whether or not it is spent, its public key
// script, and how much it pays.
type UtxoEntry struct {
	// NOTE: Additions, deletions, or modifications to the order of the
	// definitions in this struct should not be changed without considering
	// how it affects alignment on 64-bit platforms.  The current order is
	// specifically crafted to result in minimal padding.  There will be a
	// lot of these in memory, so a few extra bytes of padding adds up.

	amount      int64
	pkScript    []byte // The public key script for the output.
	blockHeight int32  // Height of block containing tx.

	// packedFlags contains additional info about output such as whether it
	// is a coinbase, whether it is spent, and whether it has been modified
	// since it was loaded.  This approach is used in order to reduce memory
	// usage since there will be a lot of these in memory.
	packedFlags txoFlags
}

// isModified returns whether or not the output has been modified since it was
// loaded.
func (entry *UtxoEntry) isModified() bool {
	return entry.packedFlags&tfModified == tfModified
}

// IsCoinBase returns whether or not the output was contained in a coinbase
// transaction.
func (entry *UtxoEntry) IsCoinBase() bool {
	return entry.packedFlags&tfCoinBase == tfCoinBase
}

// BlockHeight returns the height of the block containing the output.
func (entry *UtxoEntry) BlockHeight() int32 {
	return entry.blockHeight
}

// IsSpent returns whether or not the output has been spent based upon the
// current state of the unspent transaction output view it was obtained from.
func (entry *UtxoEntry) IsSpent() bool {
	return entry.packedFlags&tfSpent == tfSpent
}

// Spend marks the output as spent.  Spending an output that is already spent
// has no effect.
func (entry *UtxoEntry) Spend() {
	// Nothing to do if the output is already spent.
	if entry.IsSpent() {
		return
	}

	// Mark the output as spent and modified.
	entry.packedFlags |= tfSpent | tfModified
}

// Amount returns the amount of the output.
func (entry *UtxoEntry) Amount() int64 {
	return entry.amount
}

// PkScript returns the public key script for the output.
func (entry *UtxoEntry) PkScript() []byte {
	return entry.pkScript
}

// Clone returns a shallow copy of the utxo entry.
func (entry *UtxoEntry) Clone() *UtxoEntry {
	if entry == nil {
		return nil
	}

	return &UtxoEntry{
		amount:      entry.amount,
		pkScript:    entry.pkScript,
		blockHeight: entry.blockHeight,
		packedFlags: entry.packedFlags,
	}
}

// UtxoViewpoint represents a view into the set of unspent transaction outputs
// from a specific point of view in the chain.  For example, it could be for
// the end of the main chain, some point in the history of the main chain, or
// down a side chain.
//
// The unspent outputs are needed by other transactions for things such as
// script validation and double spend prevention.
type UtxoViewpoint struct {
	entries  map[wire.OutPoint]*UtxoEntry
	bestHash chainhash.Hash
}

// BestHash returns the hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) BestHash() *chainhash.Hash {
	return &view.bestHash
}

// SetBestHash sets the hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) SetBestHash(hash *chainhash.Hash) {
	view.bestHash = *hash
}

// LookupEntry returns information about a given transaction output according to
// the current state of the view.  It will return nil if the passed output does
// not exist in the view or is otherwise not available such as when it has been
// disconnected during a reorg.
func (view *UtxoViewpoint) LookupEntry(outpoint wire.OutPoint) *UtxoEntry {
	return view.entries[outpoint]
}

//addtxout adds the specified output to the view if it is not provably unspendable .
//when the view already has an entry for the outputs it will be marked unspnet. all
//fields will be updated for existing entries since it is possible it has changed during
//a reorg.
func (view *UtxoViewpoint) addTxOut(outpoint wire.OutPoint, txOut *wire.TxOut, isCoinBase bool, blockHeight int32) {

	//dont add provably unspendable outputs.
	if txscritp.IsUnspendable(txOut.PkScript) {
		return
	}

	//update existing entries ,all fields are updated it is
	//possible (although extremely unlikely)that the existing entry
	//is being replaced by a different transaction with the same hash
	//this is allowed so long as the previous trnasation is fully spent.
	entry := view.LookupEntry(outpoint)
	if entry == nil {
		entry = new(UtxoEntry)
		view.entries[outpoint] = entry

	}

	entry.amount = txOut.Value
	entry.pkScript = txOut.PkScript
	entry.blockHeight = blockHeight
	entry.packedFlags = tfModified
	if isCoinBase {
		entry.packedFlags |= tfCoinBase
	}

}

//addtxout adds teh specifed output of the passed transaction to te
//view if the exists and is not provaly unspendable. when the wive already
//has an entry for the output .it will be marked unspent .all fields will be
//updated for existing entries since it is possible it has changed druing a reorg.
func (view *UtxoViewpoint) AddTxOut(tx *btcutil.Tx, txOutIdx uint32, blockHeight int32) {
	//can not add an output for an out of bounds index.
	if txOutIdx >= uint32(len(tx.MsgTx().TxOut)) {
		return
	}

	//update existing entries .all fields are updated because it's
	// possible (although extremely unlikely) that the existing entry is
	// being replaced by a different transaction with the same hash.  This
	// is allowed so long as the previous transaction is fully spent.
	prevOut := wire.OutPoint{Hash: tx.Hash(), Index: txOutIdx}
	txOut := tx.MsgTx().TxOut[txOutIdx]
	view.addTxOut(prevOut, txOut, IsCoinBase(tx), blockHeight)

}

// AddTxOuts adds all outputs in the passed transaction which are not provably
// unspendable to the view.  When the view already has entries for any of the
// outputs, they are simply marked unspent.  All fields will be updated for
// existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOuts(tx *btcutil.Tx, blockHeight int32) {

	//loop all of the transaction outputs and add those which are not
	//provably unsendable.
	isCoinBase := IsCoinBase(tx)
	prevOut := wire.OutPoint{Hash: *tx.Hash()}

	for txOutIdx, txOut := range tx.MsgTx().TxOut {
		// Update existing entries.  All fields are updated because it's
		// possible (although extremely unlikely) that the existing
		// entry is being replaced by a different transaction with the
		// same hash.  This is allowed so long as the previous
		// transaction is fully spent.

		prevOut.Index = uint32(txOutIdx)
		view.addTxOut(prevOut, txOut, isCoinBase, blockHeight)

	}

}




// disconnectTransactions updates the view by removing all of the transactions
// created by the passed block, restoring all utxos the transactions spent by
// using the provided spent txo information, and setting the best hash for the
// view to the block before the passed block.
func (view *UtxoViewpoint) disconnectTransactions(db database.DB, block *btcutil.Block, stxos []SpentTxOut) error {
	return nil // todo
}
func (view *UtxoViewpoint) fetchInputUtxos(db database.DB, block *btcutil.Block) error {
	return nil //todo
}

// RemoveEntry removes the given transaction output from the current state of
// the view.  It will have no effect if the passed output does not exist in the
// view.
func (view *UtxoViewpoint) RemoveEntry(outpoint wire.OutPoint) {
	delete(view.entries, outpoint)
}

// Entries returns the underlying map that stores of all the utxo entries.
func (view *UtxoViewpoint) Entries() map[wire.OutPoint]*UtxoEntry {
	return view.entries
}
