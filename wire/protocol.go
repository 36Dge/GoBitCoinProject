package wire

type BitcoinNet uint32

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
