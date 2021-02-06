package main

//interruptchannel is used to receive singnal
ver interruptChannel chan os.Signal

//addhanlderchanenl is used to add an intrrupt handler to the list hanlders
//to be invoked on singanls
var addHandlerChannel = make(chan func())