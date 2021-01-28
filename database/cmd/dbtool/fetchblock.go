package main

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"encoding/hex"
	"errors"
	"time"
)

//fetchblockcmd defines the configuration options for the fetchblock command
type fetchBlockCmd struct{}

//usage overrides the usage display for the command .
func (cmd *fetchBlockCmd) Usage() string {
	return "<block-hash>"
}

//// Execute is the main entry point for the command.  It's invoked by the parser.
func (cmd *fetchBlockCmd) Execute(args []string) error {
	//setup the gloable config options and ensure they are valide.
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	if len(args) < 1 {
		return errors.New("required block hash paramether not specifiedd")
	}

	blockHash, err := chainhash.NewHashFromStr(args[0])
	if err != nil {
		return err
	}

	//load the block database
	db, err := loadBlockDB()
	if err != nil {
		return err
	}

	return db.View(func(tx database.Tx) error {
		log.Infof("fetching block %s", blockHash)
		startTime := time.Now()
		blockBytes, err := tx.FetchBlock(blockHash)
		if err != nil {
			return err
		}
		log.Infof("loaded block in %v", time.Since(startTime))
		log.Infof("block hex :%s", hex.EncodeToString(blockBytes))
		return nil
	})

}
//over
