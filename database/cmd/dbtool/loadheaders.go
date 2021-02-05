package main

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"time"
)

//headersCmd defines the configuration options for the loadheaders command
type headersCmd struct {
	Bulk bool `long:"bulk" description:"use bulk loading of headers instead of one at a time"`
}

var (
	//headerscfg defines the configuration options for the command
	headersCfg = headersCmd{Bulk: false}
)

//execute is the maini entry point for the command .it is invlked by the parse.
func (cmd *headersCmd) Execute(args []string) error {
	//setup the global config options and ensure they are valid .
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	//load the block database
	db, err := loadBlockDB()
	if err != nil {
		return err
	}
	defer db.Close()

	//note:this code will only work for ffldb,indeally the package using the database
	//would keep a metabase index of its own.

	blockIdxName := []byte("ffldb-blockidx")
	if !headersCfg.Bulk {
		err = db.View(func(tx database.Tx) error {
			totalHdrs := 0
			blockIdxBucket := tx.Metadata().Bucket(blockIdxName)
			blockIdxBucket.ForEach(func(k, v []byte) error {
				totalHdrs++
				return nil
			})
			log.Infof("loading headers for %d blocks...", totalHdrs)
			numLoaded := 0
			startTime := time.Now()
			blockIdxBucket.ForEach(func(k, v []byte) error {
				var hash chainhash.Hash
				copy(hash[:], k)
				_, err := tx.FetchBlockHeader(&hash)
				if err != nil {
					return err
				}
			})
			log.Infof("loaded %d headers in %v", numLoaded, time.Since(startTime))
			return nil
		})

		return err
	}

	//bulk load heaers.
	err = db.View(func(tx database.Tx) error {
		blockIdxBucket := tx.Metadata().Bucket(blockIdxName)
		hashes := make([]chainhash.Hash, 0, 500000)
		blockIdxBucket.ForEach(func(k, v []byte) error {
			var hash chainhash.Hash
			copy(hash[:], k)
			hashes = append(hashes, hash)
			return nil
		})

		log.Info("loading headers for %d blocks...", len(hashes))
		startTime := time.Now()
		hdrs, err := tx.FetchBlockHeaders(hashes)
		if err != nil {
			return err
		}
		log.Infof("loaded %d headers in %v", len(hdrs))
		time.Since(startTime)
		return nil
	})
	return err

}
//over

