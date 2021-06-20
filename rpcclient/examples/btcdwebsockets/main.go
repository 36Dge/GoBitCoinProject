package main

import (
	"BtcoinProject/wire"
	"github.com/btcsuite/btcutil"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"
)

func main() {

	//only override the hanlder for notifitions you care about.
	//also note most of these handler will only be called if you rigister
	//for notifications .see the documentation of the rpcclient
	//notiifcation handlertype for more detials about each hanlder.
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx) {
			log.Printf("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Printf("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
	}

	//connect to local btcd Rpc server using websockets.
	btcdHomeDir := btcutil.AppDataDir("btcd", false)
	certs, err := ioutil.ReadFile(filepath.Join(btcdHomeDir, "rpc.cert"))
	if err != nil {
		log.Fatalln(err)
	}
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:8334",
		Endpoint:     "ws",
		User:         "yourrpcuser",
		Pass:         "yourrpcpass",
		Certificates: certs,
	}

	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		log.Fatal(err)
	}

	//register for block connect and disconnet notfications .
	if err := client.NotifyBlocks(); err != nil {
		log.Fatal(err)
	}

	log.Println("notifyblocks:registercation complete")

	//get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}

	//for this ecample gracefully shutdown the client after 10 seconds
	//ordinarily when to shuttown the client is highly application
	//specific
	log.Println("client shutdown in 10 seconds")
	time.AfterFunc(time.Second*10, func() {
		log.Println("client shutting down")
		client.Shutdown()
		log.Println("client shutdown complete")
	})

	//wait until the client either shuts down gracefull (or the user
	//terminates the process with Ctrl + c)
	client.WaitForShutdown()

}
