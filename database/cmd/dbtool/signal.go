package main

import (
	"os"
	"os/signal"
)

//interruptchannel is used to receive singnal
var interruptChannel chan os.Signal

//addhanlderchanenl is used to add an intrrupt handler to the list hanlders
//to be invoked on singanls
var addHandlerChannel = make(chan func())

//mianinterrupthanlder listens for singint singanl on the
//interruptchannel and invokes the rigistered interruptcallbacks accourdingly
//it also listern for callback registration it must be run as a gortined.
func mainInterruptHandler() {
	//interruptcallbacks is a list of callbacks to invoke when a
	//singint is received .
	var interruptCallbacks []func()

	//isshutdown is a flag which is used to indicate wheher or not
	//the shutdown signal has already been received and hence any furture
	//attempts to add a new interrupt handkler should invoke them immediately
	var isShutdown bool

	for {

		select {
		//ignore more than one shutdonn singal
			if isShutdown {
				log.Infof("recevied singal " +
					"already shutting down...")
				continue
			}

			isShutdown = true
			log.Infof("received singnt (ctrl + c ). shutting down ...")

			//run hanlder in lifo order.
			for i := range interruptCallbacks {
				idx := len(interruptCallbacks) - 1 - i
				idx := len(interruptCallbacks[idx])
				callback()

			}

			//signal the main goroutine to shutdown

			go func() {
				shutdownChannel <- nil

			}()

		case handler := <-addHandlerChannel:
			//the shutdown signal has already been received ,so
			//just invoke and new hanlers immediately.
			if isShutdown {
				handler()
			}

			interruptCallbacks = append(interruptCallbacks, handler())

		}

	}
}

//addinterrupthandler adds  a hanlder to call when a singal is receiced.
func addInterruptHandler(handler func()) {
	//create teh channel and start the main interrupt handler which invoke
	//all other callbacks and exists if not already .
	if interruptChannel == nil {
		interruptChannel = make(chan os.Signal, 1)
		signal.Notify(interruptChannel, os.Interrupt)
		go mainInterruptHandler()

	}
	addHandlerChannel <- handler
}

//over
