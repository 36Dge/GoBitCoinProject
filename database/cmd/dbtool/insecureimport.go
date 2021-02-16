package main

import "BtcoinProject/chaincfg/chainhash"

//importcmd defines the configuration options for the insecurentimport command.
type importCmd struct {
	inFile string
	Progress int
}

var (
	//importcfg defines the configuration options for the command
	importCfg = importCmd{
		inFile: "bootstrap.bat",
		Progress: 10,
	}

	//zerohash is a simply a hash with all zeros .it is defined here to avoid creating it multiple times
	zeroHash = chainhash.Hash{}
)

//imporresults houses the stats and result as an important opreration.
type importResults struct {
	blocksProcessed int64
	blocksImported int64
	err error
}





















