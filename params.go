package main

//activenetparams是指向特定于当前活跃的比特币网络。

var activeNetParams = &mainNetParams

//参数用于对各种网络的参数进行分组，例如网络和测试网络

type params struct {
	*chaincfg.Params
	rpcPort string
}

//mainnetparams包含特定于主网络的参数 Wire.Mainnet）。注意：rpc端口有意与
//参考实现，因为BTCD不处理钱包请求。这个单独的钱包进程监听已知端口并转发请求
//它不能处理BTCD。这种方法允许钱包过程以模拟完整引用实现rpc api。
var mainNetParams = params{
	Params:  &chaincfg.MainNetParams,
	rpcPort: "8334",
}

//regressionnetparams包含特定于回归测试的参数网络（Wire.TestNet）。注意：rpc
//端口故意不同而不是引用实现-有关细节。
var regerssionNetParams = params{
	Params:  &chaincfg.RegressionNetParams,
	rpcPort: "18334",
}

//testnet3参数包含特定于测试网络的参数（版本3）（Wire.TestNet3）。注意：rpc端
//口,有意与参考实现-有关详细信息，请参阅mainnetparams注释。
var testNet3Params = params{
	Params:  &chaincfg.TestNetParams,
	rpcPort: "18334",
}

//simnetparams包含特定于模拟测试网络的参数（电线，SimNet）
var simNetParams = params{
	Params:  &chaincfg.SimNetParams,
	rpcPort: "18556",
}

//netname返回引用比特币网络时使用的名称。在写入时间，btcd当前将testnet版本3的块放在
//数据和日志目录“testnet”，与CHAINCFG参数。此函数可用于重写此目录
//当传递的活动网络与Wire.TestNet3匹配时，将其命名为“testnet”。
func netName(chainParams *params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}

//over