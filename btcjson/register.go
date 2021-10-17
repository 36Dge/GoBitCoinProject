package btcjson

import "fmt"

//mustregistercmd perform the same function as registercmd expect it panics
//if there is an error. this should only be called form package init functions.
func MustRegisterCmd (method string ,cmd interface{},flags UsageFlag) {
	if err := RegisterCmd(method,cmd,flags); err != nil {
		panic(fmt.Sprintf("failed to register type %q:%v\n",method,err))
	}
}
