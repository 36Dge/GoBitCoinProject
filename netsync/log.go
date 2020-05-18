package netsync

import "github.com/btcsuite/btclog"

// 日志是一个没有输出过滤器初始化的日志程序。
// 这个意味着在调用方之前，包默认不会执行任
// 何日志记录请求
var log btclog.Logger

//DisableLog禁用所有库日志输出。日记记录输出
//被禁用，默认情况下，直到调用uselogger或set
//logwriter。
func DisableLog() {
	log = btclog.Disabled
}

// uerlogger使用指定的记录器输出包日志信息
// 如果调用方还使用BTCULG
func UseLogger(logger btclog.Logger) {
	log = logger
}









