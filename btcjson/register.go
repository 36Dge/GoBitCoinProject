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
	registerLock         sync.RWMutex
	methodToConcreteType = make(map[string]reflect.Type)
	methodToInfo         = make(map[string]methodInfo)
	concreteTypeToMethod = make(map[reflect.Type]string)
)

//basekingstring returns the base kind for a given refeect.type after
//indirecting through all pointer.
func baseKingString(rt reflect.Type) string {
	numIndirects := 0
	for rt.Kind() == reflect.Ptr {
		numIndirects++
		rt = rt.Elem()
	}

	return fmt.Sprintf("%s%s", strings.Repeat("*", numIndirects), rt.Kind())
}

//isacceutbaleking returns wheher or not be passed field type is a supported
//type .it is called after the first pointer indriction.so further pointers
//are not supported.

func isAcceptableKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan:
		fallthrough
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Ptr:
		fallthrough
	case reflect.Interface:
		return false
	}

	return true
}

//registercmd registers a new command that will automatically marshall to and
//from json-prc with full type checking and positional parameter support. it
//aslo accepts usage flags which identify the circumstances under which hte
//command can be used.

//this package automatically registers all of the exported commands by
//default using this function ,however it is also exported so callers can
//esaily register custom types.

//the type fomat is very strict since it needs to be albe to automatically
//marshal to and from json-rpc 1.0 the following enumrates the requirements

//the provided command must be s single pointer to a struct
//all fields must be exported.
//the order of the positional parameters in the marshalled json will be in /
// the same order as declared in the struct definition.

//struct embeding is not supported.
//struct fielss may not be channels ,functions ,complex,or interface
//a field in the provided struct with a pointer is treated as optional
//multiple indirections are not supported.

//Once the first optional field (pointer) is encountered, the remaining
// fields must also be optional fields (pointers) as required by positional
// params
// A field that has a 'jsonrpcdefault' struct tag must be an optional field
//  (pointer)

//note:this function only needs to be able to examine the structre fo the
//passed struct. so it does need to be an actual instance .therefore.
//it is recommanded to simply pass a nil pointer cast to the appropriate type .
//for example.(foocmd)(nil)

func RegisterCmd (method string ,cmd interface{},flags UsageFlag) error{
	registerLock.Lock()
	defer registerLock.Unlock()

	if _,ok := methodToConcreteType[method];ok {
		str := fmt.Sprintf("method %q is already registed",method)
		return makeError(ErrDuplicateMethod,str)
	}

	//ensure that no unrecongized flag bits were speicfied.
	if ^(highestUsageFlagBit-1) & flags != 0 {
		str := fmt.Sprintf("invalid usage flags speicifed for method "+
			"%s:%v",method,flags)
		return makeError(ErrInvalidUsageFlags, str)
	}

	rtp := reflect.TypeOF(cmd)
	if rtp.Kind() != reflect.Ptr {
		str := fmt.Sprintf("type must be *struct not '%s(%s)'",rtp,rtp.kind())
		return makeError(ErrInvalidType,str)
	}

	rt := rtp.Elem()
	if rt.Kind() != reflect.Struct {
		str := fmt.Sprintf("type must be *struct not '%s(*%s)'",rtp,rt.Kind())
		return makeError(ErrInvalidType,str)
	}











}














//mustregistercmd perform the same function as registercmd expect it panics
//if there is an error. this should only be called form package init functions.
func MustRegisterCmd(method string, cmd interface{}, flags UsageFlag) {
	if err := RegisterCmd(method, cmd, flags); err != nil {
		panic(fmt.Sprintf("failed to register type %q:%v\n", method, err))
	}
}
