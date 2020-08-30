package main

import (
	"os"
	"syscall"
)

func init() {
	interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
}


//over