package mempool

//日志是一个没有输出过滤器初始化的日志程序。这个
//意味着在调用方之前，包默认不会执行任何日志记录
//请求它。

import "github.com/btcsuite/btclog"

var log btclog.Logger

//默认的日志记录量为无
func init() {
	DisableLog()
}

//DisableLog禁用所有库日志输出。日志记录输出被禁用
//默认情况下，直到调用uselogger或setlogwriter。

func DisableLog() {
	log = btclog.Disabled
}

//uselogger使用指定的记录器输出包日志信息。
//如果调用方还
//使用BTCULG。

func UseLogger(logger btclog.Logger) {
	log = logger
}

//picknoun返回名词的单数或复数形式，具体取决于
//在计数上

func pickNoun(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural

}

// 本文件结束

