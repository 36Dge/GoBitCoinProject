package btcjson

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

//usageflags define flags that specify additional properties about the
//circumstances under which a command can be used.
type UsageFlag uint32

const (
	//ufwalletonly indicates that the command can only be used with an PRC
	//server that supports wallet commands.
	UFWalletOnly UsageFlag = 1 << iota

	//ufwebsocketsonly indicates that the command can only be used when
	//communicationg with an RPC server over websockts .this typeically
	//applies to notifications and notification registratijon functnions.
	//since neither makes since when using a single-shot http_post request.
	UFWebsocketOnly
	//ufnoticaition indicates that the command is actually a notification.
	//this means when it is marshalled.the ID must be nil.
	UFNotification

	//higestUsagflagbit is the maximum usage flag bit and is used in the
	//stingger and tests to ensure all of the above constants have been
	//tested.
	highestUsageFlagBit
)

//map of usageflag vulues back to their constant names for prerry printing
var usageFlagStrings = map[UsageFlag]string{
	UFWalletOnly:    "UFWalletOnly",
	UFWebsocketOnly: "UFWebsocketOnly",
	UFNotification:  "UFNotification",
}

//string returns the usagflag in human-readable form.
func (fl UsageFlag) String() string {
	//no flags are set.
	if fl == 0 {
		return "0x0"
	}

	//add indivvidual bit falgs.
	s := ""
	for flag := UFWalletOnly; flag < highestUsageFlagBit; flag <<= 1 {
		if fl&flag == flag {
			s += usageFlagStrings[flag] + "|"
			fl -= flag
		}
	}

	//add remaining value as raw hex.
	s = strings.TrimRight(s, "|")
	if fl != 0 {
		s += "|0x" + strconv.FormatUint(uint64(fl), 16)
	}
	s = strings.TrimLeft(s, "|")
	return s

}

//methodinfo keeps track of information about each registered method such as
//the parameter informantion
type methodInfo struct {
	maxParams    int
	numReqParams int
	numOptParams int
	defaults     map[int]reflect.Value
	flags        UsageFlag
	usage        string
}

var (
	//this fields are used to map the registed types to method names.
	registerLock sync.RWMutex
	methodToConcreteType = make(map[string]reflect.Type)
	methodToInfo = make(map[string]methodInfo)
	concreteTypeToMethod = make(map[reflect.Type]string)
)


//mustregistercmd perform the same function as registercmd expect it panics
//if there is an error. this should only be called form package init functions.
func MustRegisterCmd(method string, cmd interface{}, flags UsageFlag) {
	if err := RegisterCmd(method, cmd, flags); err != nil {
		panic(fmt.Sprintf("failed to register type %q:%v\n", method, err))
	}
}
