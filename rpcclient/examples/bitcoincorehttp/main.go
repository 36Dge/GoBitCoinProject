package main

import (

	"log"
	"github.com/btcsuite/btcd/rpcclient"
)

func main() {

	//connect to local bitcoin core rpc server using http post mode
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:8332",
		User:         "yourrpcuser",
		Pass:         "yourrpcpass",
		HTTPPostMode: true, //bitcore core only supporrts http post mode
		DisableTLS:   true, // bitcoin core does not provide tls by deault.
	}

	//notice th notcification paramater is nil since notification are not supported
	//in http post mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Shotdown()

	//get the current block count.
	blockCount, err := client.getBlockCount()
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("block count :%d", blockCount)

}
