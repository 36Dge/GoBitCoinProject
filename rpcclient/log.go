package rpcclient

import "github.com/btcsuite/btclog"

//log is a logger that is initialized with no output filter ,this means
//the package will not perform any loggging by default until the caller
//request it.
var log btclog.Logger

//the default amount of logging is none.
func init() {
	DisableLog()
}

//disableLog disable all library log output .logging output is disabled
//by default until useLogger is called.
func DisableLog(){
	log = btclog.Disabled
}


//useLogger uses a special logger to output package logging info
func UserLogger(logger btclog.Logger) {
	log = logger
}

//logclosure is a clouse that can be printed with %v to be used to
//generate expensive-to-create data for a detailed log level and avoid
//doing the work if the data isn,t printed
type logClosure func() string

//string invokes the log closure and returns the results string.
func(c logClosure)String() string{
	return c()

}

// newLogClosure returns a new closure over the passed function which allows
// it to be used as a parameter in a logging function that is only invoked when
// the logging level is such that the message will actually be logged.
func newLogClosure(c func() string) logClosure {
	return logClosure(c)
}

//over




















