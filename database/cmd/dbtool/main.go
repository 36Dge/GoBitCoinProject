package main

import (
	"BtcoinProject/database"
	"github.com/btcsuite/btclog"
	"os"
	"path/filepath"
)

const (

	//blockdbnameprefix is the prefix for the btcd block database
	blockDbNamePrefix = "blocks"
)

var (
	log             btclog.Logger
	shutdownChannel = make(chan error)
)

//loadblockdb opens the block database and reutrns a handle to it
func loadBlockDB() (database.DB, error) {

	//the database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + cfg.DbType
	dbPath := filepath.Join(cfg.DataDir, dbName)

	log.Infof("loading block database from '%s'", dbPath)
	db, err := database.Open(cfg.DbType, dbPath, activeNetParams.Net)
	if err != nil {
		//return the error if it is not because the database do inn,t
		//eixst
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {
			return nil, err
		}

		//create the  db if it does not exist
		err = os.Mkdir(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}

		db, err = database.Create(cfg.DbType, dbPath, activeNetParams.Net)
		if err != nil {
			return nil, err
		}

	}

	log.Info("block database loaded")
	return db, nil

}














