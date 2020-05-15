package main

import (
	"github.com/btcsuite/btclog"
	"github.com/jrick/logrotate/rotator"
	"os"
)

//LogWriter实现了一个IO.Writer，它同时输出到标准输出和初始化的日志旋转器的写入结束管道。

type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	logRotator.Write(p)
	return len(p), nil
}

//每个子系统的记录器。创建一个后端记录器，并创建所有子系统从中创建的记录器将写入后端。添加
//新子系统，将subsystem logger变量添加到子系统记录器映射。在用初始化日志旋转器之前，不能
//使用记录器日志文件。必须在应用程序启动的早期通过调用内旋转器.

var (
	//backendlog是用于创建所有子系统记录器的日志后端。在日志旋转器初始化之前，不能使用后端，否
	//则将发生数据争用和/或零指针取消引用。

	backendLog = btclog.NewBackend(logWriter{})

	//logrotator是日志输出之一。它应该关闭,应用程序关闭。
	logRotator *rotator.Rotator

	adxrLog = backendLog.Logger("ADXR")
	amgrLog = backendLog.Logger("AMGR")
	cmgrLog = backendLog.Logger("CMGR")
	bcdbLog = backendLog.Logger("BTCD")



)
