package main

//activenetparams是指向特定于当前活跃的比特币网络。

var activeNetParams = &mainNetParams

//参数用于对各种网络的参数进行分组，例如网络和测试网络

type params struct {
	*chaincfg.Params
	rpcPort string
}

