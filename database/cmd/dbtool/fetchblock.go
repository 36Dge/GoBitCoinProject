package main

//fetchblockcmd defines the configuration options for the fetchblock command
type fetchBlockCmd struct {}

//usage overrides the usage display for the command .
func(cmd *fetchBlockCmd)Usage() string{
	return "<block-hash>"
}

//// Execute is the main entry point for the command.  It's invoked by the parser.
