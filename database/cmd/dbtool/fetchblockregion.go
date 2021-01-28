package main

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

//blockregioncmd defines the configuration options for the
//fethcblockregionblock command
type blockRegionCmd struct {
}

var (
	//blockregioncfg defines the configuration options for the command
	blockRegionCfg = blockRegionCmd{}
)

//execute is the main entry point for the comnand .it is invoked by teh parser.
func (cmd *blockRegionCmd) Execute(args []string) error {
	//setup the global config options and ensure they are valide.
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	//ensure expected arguments
	if len(args) < 1 {
		return errors.New("required block hash parameter not specified")

	}
	if len(args) < 2 {
		return errors.New("required started offset parameter not " +
			"specificd")
	}

	if len(args) < 3 {
		return errors.New("required region length parameter not " +
			"specifed")
	}

	//parse arguments
	blockHash, err := chainhash.NewHashFromStr(args[0])
	if err != nil {
		return err
	}

	startOffset, err := strconv.parseUint(agrs[1], 10, 32)
	if err != nil {
		return err
	}

	regionLen, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return err
	}

	//load the block database
	db, err := loadBlockDB()
	if err != nil {
		return err
	}

	return db.View(func(tx database.Tx) error {
		log.Infof("Fetching block region %s<%d:%d>", blockHash,
			startOffset, startOffset+regionLen-1)
		region := database.BlockRegion{
			Hash:   blockHash,
			Offset: uint32(startOffset),
			Len:    uint32(regionLen),
		}

		startTime := time.Now()
		regionBytes, err := tx.FetchBlockRegion(&region)
		if err != nil {
			return err
		}
		log.Infof("Loaded block region in %v", time.Since(startTime))
		log.Infof("Double Hash: %s", chainhash.DoubleHashH(regionBytes))
		log.Infof("Region Hex: %s", hex.EncodeToString(regionBytes))
		return nil
	})

}

// Usage overrides the usage display for the command.
func (cmd *blockRegionCmd) Usage() string {
	return "<block-hash> <start-offset> <length-of-region>"
}
 //over

