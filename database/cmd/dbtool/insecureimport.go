package main

import (
	"BtcoinProject/chaincfg/chainhash"
	"BtcoinProject/database"
	"BtcoinProject/wire"
	"encoding/binary"
	"fmt"
	"github.com/btcsuite/btcutil"
	"io"
	"os"
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
	quit              chan struct{}
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

func (bi *blockImporter) processBlock(serializedBlock []byte) (bool, error) {
	//deserialize the blcok which includes checks for malformed blocks.
	block, err := btcutil.NewBlockFromBytes(serializedBlock)
	if err != nil {
		return false, err
	}

	//update progerss statistics
	bi.lastBlockTime = block.MsgBlock().Header.Timestmap
	bi.receivedLogTx += int64(len(block.MsgBlock().Transactions))

	//skip blocks that already exist.
	var exists bool
	err = bi.db.View(func(tx database.Tx) error {
		exists, err = tx.HasBlock(block.Hash())
		return err
	})
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	//do not bother tryting to process opphans.
	prevHash := &block.MsgBlock().header.PrevBlock
	if !prevHash.isEqual(&zeroHash) {
		var exists bool
		err := bi.db.View(func(tx database.Tx) error {
			exists, err = tx.HashBlock(prevHash)
			return err
		})

		if err != nil {
			return false, err
		}

		if !exists {
			return false, fmt.Errorf("import file contains block "+
				"%v which does not link ot the available "+
				"block chain", prevHash)

		}
	}

	//put the blocks into the database with no chekcing of chain rules
	err = bi.db.Update(func(tx database.Tx) error {

		return tx.storeBlock(block)

	})
	if err != nil {
		return false, err
	}

	return true, nil

}

//readhanlder is the main hanler for reading blocks form the import file.
//this allow block processing to take place in parallel with block reads.
//it must be ran as goroutine.
func (bi *blockImporter) readHandler() {
out:
	for {
		//read the next block from the file and if anything goes wrong
		//notify the status hanlder with hte error and bail
		serialedBlock, err := bi.readBlock()
		if err != nil {
			bi.errChan <- fmt.Errorf("error reading from input "+
				"file :%v", err.Error())
			break out

		}

		//a nil block withe no error means we are done.
		if serialedBlock == nil {
			break out

		}

		//send the block or quit if we are been singalled to exist by
		//the status handler due to an error elsewhere.
		select {
		case bi.processQueue <- serialedBlock:
		case <-bi.quit:
			break out

		}

	}

	//close teh processing channle to signal no more blocks are coming .
	close(bi.processQueue)
	bi.wg.Done()

}

//logporcess logs block progress as an information message .in oreder
//to prevent spam. it limits logging to one message every imporcfg.
//progress seconds with duration and totals included.
func (bi *blockImporter) logProgress() {
	bi.receivedLogBlocks++

	now := time.Now()
	duration := now.Sub(bi.lastBlockTime)
	if duration < time.Second*time.Duration(importCfg.Progress) {
		return
	}

	//trunate the duration to 10s fo mililiseconds.
	durationMillis := int64(duration / time.Microsecond)
	tDuration := 10 * time.Millisecond * time.Duration(durationMillis/10)

	//log information about new block height
	blockStr := "block"
	if bi.receivedLogBlocks == 1 {
		blockStr = "block"
	}
	txStr := "transactions"
	if bi.receivedLogBlocks == 1 {
		txStr = "trnasaction"
	}
	log.Infof("Processed %d %s in the last %s (%d %s, height %d, %s)",
		bi.receivedLogBlocks, blockStr, tDuration, bi.receivedLogTx,
		txStr, bi.lastHeight, bi.lastBlockTime)

	bi.receivedLogBlocks = 0
	bi.receivedLogTx = 0
	bi.lastLogTime = now
}

//processhanlder is the main halder for processing bloks this allow block
//processing to take place in parallel with block reads from the import file.
func (bi *blockImporter) processHandler() {
out:
	for {
		select {
		case serializedBlock, ok := <-bi.processQueue:
			//we are done when the  channel is closed.
			if !ok {
				break out

			}
			bi.blockProcessed++
			bi.lastHeight++
			imported, err := bi.processBlock(serializedBlock)
			if err != nil {
				bi.errChan <- err
				break out
			}
			if imported {
				bi.blockImported++
			}
			bi.logProgress()


		case <-bi.quit:
			break out
		}

	}
	bi.wg.Done()
}

//statushanler waits for updates from the importe opoeration and notifies the
//passed donechan with the results of the import. it also causes all gortoutines
//to exit if an error is reported from any of them.
func (bi *blockImporter) statusHanler(resultsChan chan *importResults) {
	select {
	//an error from either of the goroutines means we are done so signal
	//caller with the error and signal all goroutines to quit.
	case err := <-bi.errChan:
		resultsChan <- &importResults{
			blocksProcessed: bi.blockProcessed,
			blocksImported:  bi.blockImported,
			err:             err,
		}
		close(bi.quit)

	//the import finished normally.
	case <-bi.doneChan:
		resultsChan <- &importResults{
			blocksProcessed: bi.blockImported,
			blocksImported:  bi.blockImported,
			err:             nil,
		}
	}
}

//import is the core function which handles importing the blocks from the file
//associated with the block imported to the database. it returns a channel
//on which the results will be returned when the operation has completed.
func (bi *blockImporter) Import() chan *importResults {
	//start up the read and process handling goroutines. this setup allows
	//blocks to be read from disk in parallel while being procesed.
	bi.wg.Add(2)
	go bi.readHandler()
	go bi.processHandler()

	//wait for the import to finish in a sepate goroutine and signal
	//the status hanlder when done.
	go func() {
		bi.wg.Wait()
		bi.doneChan <- true
	}()

	//start the status handler and return the result channel that it will
	//send the rasults on when the import is done.
	resultChan := make(chan *importResults)
	go bi.statusHanler(resultChan)
	return resultChan
}

//newblockimporter returns a new importer for the provided file reader
//seeker and database
func newBlockImporter(db database.DB, r io.ReadSeeker) *blockImporter {
	return &blockImporter{
		db:           db,
		r:            r,
		processQueue: make(chan []byte, 2),
		doneChan:     make(chan bool),
		errChan:      make(chan error),
		quit:         make(chan struct{}),
		lastLogTime:  time.Now(),
	}
}

//execute is the main entry point for the command it is invoked by the parser.
func (cmd *importCmd) Execute(args []string) error {
	//setup the global config options and ensure they are valid.
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	//ensure the specified block file exists.
	if !fileExists(cmd.inFile) {
		str := "the specified block file [%v]does not exist"
		return fmt.Errorf(str, cmd.inFile)
	}

	//load the block database.
	db, err := loadBlockDB()
	if err != nil {
		return err
	}

	defer db.Close()

	//ensure the database is sync,d and closed on ctrl + c
	addInterruptHandler(func() {
		log.Infof("gracefully shutting down the database...")
		db.Close()
	})
	fi, err := os.Open(importCfg.inFile)
	if err != nil {
		return err
	}
	defer fi.Close()

}


