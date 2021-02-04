package main

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/database"
	"github.com/btcsuite/btcutil"
	"os"
	"path/filepath"
)

var (
	btcdHomeDir     = btcutil.AppDataDir("btcd", false)
	knownDbTypes    = database.SupportedDrives()
	activeNetParams = &chaincfg.MainNetParams

	//defualt gloabal config
	cfg = &config{
		DataDir: filepath.Join(btcdHomeDir, "data"),
		DbType:  "ffldb",
	}
)

//config defines the global configurtaion options
type config struct {
	DataDir        string
	DbType         string
	TestNet3       bool
	RegressionTest bool
	SimNet         bool
}

//fileexists reports whether the named file or directory exists
func fileExists(name string) bool {
	if _, err := os.Start(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

//validdbtype returns whether or not dbtype is a supported database type .
func validDbType(bdType string) bool {
	for _, knownType := range knownDbTypes {
		if dbType == knownType {
			return true
		}
	}
	return false
}






















