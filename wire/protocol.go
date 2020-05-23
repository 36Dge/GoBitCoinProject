package wire

import (
	"fmt"
	"strconv"
	"strings"
)

type BitcoinNet uint32

const (
	//protocolversion is the latest portocol version this package suppot
	ProtocolVersion uint32 = 70013

	// MultipleAddressVersion is the protocol version which added multiple
	// addresses per message (pver >= MultipleAddressVersion).
	MultipleAddressVersion uint32 = 209

	// NetAddressTimeVersion is the protocol version which added the
	// timestamp field (pver >= NetAddressTimeVersion).
	NetAddressTimeVersion uint32 = 31402

	// BIP0031Version is the protocol version AFTER which a pong message
	// and nonce field in ping were added (pver > BIP0031Version).
	BIP0031Version uint32 = 60000

	// BIP0035Version is the protocol version which added the mempool
	// message (pver >= BIP0035Version).
	BIP0035Version uint32 = 60002

	// BIP0037Version is the protocol version which added new connection
	// bloom filtering related messages and extended the version message
	// with a relay flag (pver >= BIP0037Version).
	BIP0037Version uint32 = 70001

	// RejectVersion is the protocol version which added a new reject
	// message.
	RejectVersion uint32 = 70002

	// BIP0111Version is the protocol version which added the SFNodeBloom
	// service flag.
	BIP0111Version uint32 = 70011

	// SendHeadersVersion is the protocol version which added a new
	// sendheaders message.
	SendHeadersVersion uint32 = 70012

	// FeeFilterVersion is the protocol version which added a new
	// feefilter message.
	FeeFilterVersion uint32 = 70013
)

// serviceflag identifies services supported by a bitcion peer
type ServiceFlag uint64

const (
	//sfnodenetwork is a flag used to indicate a peer is a full node
	SFNodeNetwork ServiceFlag = 1 << iota

	//sfnodegetutxo is flag used to indicate a peer supports the getutxos
	// utxos commands(bip0064)
	SFNodeGetUTXO

	//sfnodebloom is a falg used to indicate a peer support bloom filtering
	SFNodeBloom

	//sfnodewitness is a flag used to indicate a peer supports blocks
	//and transactions including witness data (bip0144)
	SFNodeWitness

	//sfnodexthin is a flag used to indicate a peer supports xthin blocks
	SFNodeXthin

	//sfnodebit5 is a flag used to indicate a peer support a service difined
	//by bit5

	SFNodeBit5

	//sfnodecf is a flag used to indicate a peer supports committed
	// filters(CFs)
	SFNodeCF

	//sfnode2x is a falg used to indicate a peer is running the segwit2x software
	SFNode2X
)

// map of service flags back to their constant names for  pretty print
var sfStrings = map[ServiceFlag]string{

	SFNodeNetwork: "SFNodeNetwork",
	SFNodeGetUTXO: "SFNodeGetUTXO",
	SFNodeBloom:   "SFNodeBloom",
	SFNodeWitness: "SFNodeWitness",
	SFNodeXthin:   "SFNodeXthin",
	SFNodeBit5:    "SFNodeBit5",
	SFNodeCF:      "SFNodeCF",
	SFNode2X:      "SFNode2X",
}

//orderedsfstrings is an ordered list of service flags from highest to
// lowest
var orderedSFStrings = []ServiceFlag{

	SFNodeNetwork,
	SFNodeGetUTXO,
	SFNodeBloom,
	SFNodeWitness,
	SFNodeXthin,
	SFNodeBit5,
	SFNodeCF,
	SFNode2X,
}

// strings returns the serviceflag in human-readable form
func (f ServiceFlag) String() string {
	//no flags are set
	if f == 0 {
		return "0x0"
	}
	//add indaividual bit flags
	s := ""
	for _, flag := range orderedSFStrings {
		if f&flag == flag {
			s += sfStrings[flag] + "|"
			f -= flag
		}
	}
	//add any remaining flags which aren't accounted for as hex
	s = strings.TrimRight(s, "|")
	if f != 0 {
		s += "|0x" + strconv.FormatUint(uint64(f), 16)
	}
	s = strings.TrimLeft(s, "|")
	return s
}

//用于指示消息比特币网络的常量。他们也可以
//用于在流的状态未知时查找下一条消息，但
//此包不提供该功能，因为它通常更好的办法是简
//单地断开那些在TCP上行为不端的客户机。

const (
	//mainnet代表主要比特币网络
	MainNet BitcoinNet = 0xd9b4bef9

	//testnet表示回归测试网络。
	TestNet BitcoinNet = 0xdab5bffa

	//TestNet3表示测试网络（版本3）。
	TestNet3 BitcoinNet = 0x0709110b

	//simnet表示模拟测试网络
	SimNet BitcoinNet = 0x12141c16
)

// bnstrings is a map of bitcoin newwork back to their constant names for
// pretty printing.
var bnStrings = map[BitcoinNet]string{
	MainNet:  "MainNet",
	TestNet:  "TestNet",
	TestNet3: "TestNet3",
	SimNet:   "SimNet",
}

// string returns the bitcoinNet in human-readable form
func (n BitcoinNet) String() string {
	if s, ok := bnStrings[n]; ok {
		return s
	}
	return fmt.Sprintf("Unknown BitcoinNet (%d)", uint(n))
}

//over
