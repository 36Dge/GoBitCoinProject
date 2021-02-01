package main

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/database"
	"github.com/btcsuite/btcutil"
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
	DataDir string
	DbType string
	TestNet3 bool
	RegressionTest bool
	SimNet bool
}
