package main

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
)

//importcmd defines the configuration options for the insecurentimport command.
type importCmd struct {
	inFile   string
	Progress int
}

var (
	//importcfg defines the configuration options for the command
	importCfg = importCmd{
		inFile:   "bootstrap.bat",
		Progress: 10,
	}

	//zerohash is a simply a hash with all zeros .it is defined here to avoid creating it multiple times
	zeroHash = chainhash.Hash{}
)

//imporresults houses the stats and result as an important opreration.
type importResults struct {
	blocksProcessed int64
	blocksImported  int64
	err             error
}

//blockimporter houses information about an ongoing imprott form a block data
//file to the block database.
type blockImporter struct {
	db                database.DB
	r                 io.ReadSeeker
	processQueue      chan []byte
	doneChan          chan bool
	errChan           chan error
	wg                sync.WaitGroup
	blockProcessed    int64
	blockImported     int64
	receivedLogBlocks int64
	receivedLogTx     int64
	lastHeight        int64
	lastBlockTime     time.Time
	lastLogTime       time.Time
}

//readblock reads the next block from the input file.
func (bi *blockImporter) readBlock() ([]byte, error) {
	//the block file format is :
	//<network> <block length > <serialized block>
	var net uint32
	err := binary.Read(bi.r, binary.LittleEndian, &net)
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
		//no block and no error means there are no more blocks to read.
		return nil, nil
	}

	if net != uint32(activeNetParams.Net) {
		return nil, fmt.Errorf("network mismatch -- got %x,want %x", net,
			uint32(activeNetParams.Net))

	}

	//read the block length and ensure it is sane.
	var blockLen uint32
	if err := binary.Read(bi.r, binary.LittleEndian, &blockLen); err != nil {
		return nil, err
	}
	if blockLen > wire.MaxBlockPayload {
		return nil, fmt.Errorf("block payload of %d bytes is larger"+
			"than the max allowed %d bytes", blockLen, wire.MaxBlockPayload)

	}

	serializedBlock := make([]byte, blockLen)
	if _, err := io.ReadFull(bi.r, serializedBlock); err != nil {
		return nil, err
	}

	return serializedBlock, nil

}

// processBlock potentially imports the block into the database.  It first
// deserializes the raw block while checking for errors.  Already known blocks
// are skipped and orphan blocks are considered errors.  Returns whether the
// block was imported along with any potential errors.
//
// NOTE: This is not a safe import as it does not verify chain rules.

