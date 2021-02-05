package main

import (
	"BtcoinProject/chaincfg"
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"errors"
	"fmt"
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

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, btcd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory name
// as "testnet" when the passed active network matches wire.TestNet3.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet3" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.

func netName(chainParams *chaincfg.Params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"

	default:
		return chainParams.Name
	}
}

//setupglobalconfig examine the global configruation optional for any contions which are
//invalid as well as performs any additional setup necessary after the initial parse.
func setupGlobalConfig() error {
	//multiple newwork can be selected simultaneously
	//count number of network flags pased;assing active network parses
	//while we are at it
	numNets := 0
	if cfg.TestNet3 {
		numNets++
		activeNetParams = &chaincfg.TestNet3Params
	}
	if cfg.RegressionTest {
		numNets++
		activeNetParams = &chaincfg.ResgressionNetParams
	}
	if cfg.SimNet {
		numNets++
		activeNetParams = &chaincfg.SimNetParams
	}

	if numNets > 1 {
		return errors.New("The testnet, regtest, and simnet params " +
			"can't be used together -- choose one of the three")
	}

	//validate database type .
	if !validDbType(cfg.DbType){
		str := "The specified database type [%v] is invalid -- " +
			"supported types %v"
		return fmt.Errorf(str,cfg.DbType,knownDbTypes)
	}

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	cfg.DataDir = filepath.Join(cfg.DataDir, netName(activeNetParams))

	return nil
}

//over



















