package blockchain

import "BtcoinProject/chaincfg/chainhash"

//behaviorflag is a bitmask difining tweaks to the normal behavior when performing
//chain processing and consesus rules checks.
type BehaviorFlags uint32

const (
	//bffastadd may be set to indicate that several checks can be avioded for the block since it is
	//already known to fit into the chain due to already proving it corretcint links
	//into the chain up to a known checkpoint .this is primaryly used for headers-ifrst mode.
	BFFastAdd BehaviorFlags = 1 << iota

	//bfnoppowcheck may be set to indicate the proof of work check which ensure
	//a block hashed to a value less than the required target will not be performed.
	BFNoPoWCheck

	//bfnone is a convenience value to specifically indicate no flags
	BFNone BehaviorFlags = 0
)

//blockexists determains whether a block with the given hash exists either in the main /
//chain or any side chains .

//this function is safe for concurrent access.
func (b *BlockChain) blockExists(hash *chainhash.Hash) (bool, error) {
	//check block index first (cound be main chain or side chain blocks)
	if b.index.HaveBlock(hash) {
		return true, nil
	}

	//check in the database.
	var exists bool
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		exists, err = dbTx.HasBlock(hash)
		if err != nil || !exists {
			return err
		}

		//ingore side chain blocks in the database.this is necessary because
		//there is not currently any recode of the associated block index data
		//such as ites block height .so it is not yet possible to effiently load
		//the block and do anything useful with it .
		//ultimately the entire block index should be serialized instead of only the curent
		//main so it can be consulted directly.
		_, err = dbFetchHeightByHash(dbTx, hash)
		if isNotInMainChainErr(err) {
			exists = false
			return nil
		}

		return err

	})

	return exists, err
}






















