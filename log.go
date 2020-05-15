package main

import (
	"BtcoinProject/mempool"
	"fmt"
	"github.com/btcsuite/btclog"
	"github.com/jrick/logrotate/rotator"
	"os"
	"path/filepath"
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
	//backendlog是用于创建所有子系统记录器的日志后端。在日志旋转器初始化之前，不能使用后端，
	//否则将发生数据争用和/或零指针取消引用。

	backendLog = btclog.NewBackend(logWriter{})

	//logrotator是日志输出之一。它应该关闭,应用程序关闭。
	logRotator *rotator.Rotator

	adxrLog = backendLog.Logger("ADXR")
	amgrLog = backendLog.Logger("AMGR")
	cmgrLog = backendLog.Logger("CMGR")
	bcdbLog = backendLog.Logger("BTCD")
	btcdLog = backendLog.Logger("BTCD")
	chanLog = backendLog.Logger("CHAN")
	discLog = backendLog.Logger("DISC")
	indxLog = backendLog.Logger("INDX")
	minrLog = backendLog.Logger("MINR")
	peerLog = backendLog.Logger("PEER")
	rpcsLog = backendLog.Logger("RPCS")
	scrpLog = backendLog.Logger("SCRP")
	srvrLog = backendLog.Logger("SRVR")
	syncLog = backendLog.Logger("SYNC")
	txmpLog = backendLog.Logger("TXMP")
)

// 初始化包全局记录器变量
func init() {
	addrmgr.UseLogger(amgrLog)
	connmgr.UseLogger(cmgrLog)
	database.UseLogger(bcdbLog)
	blockchain.UseLogger(chanLog)
	indexers.UseLogger(indxLog)
	mining.UseLogger(minrLog)
	cpuminer.UseLogger(minrLog)
	peer.UseLogger(peerLog)
	txscript.UseLogger(scrpLog)
	netsync.UseLogger(syncLog)
	mempool.UseLogger(txmpLog)
}

//子系统记录器将每个子系统标识符映射到其关联的记录器。
var subsystemLoggers = map[string]btclog.Logger{
	"ADXR": adxrLog,
	"AMGR": amgrLog,
	"CMGR": cmgrLog,
	"BCDB": bcdbLog,
	"BTCD": btcdLog,
	"CHAN": chanLog,
	"DISC": discLog,
	"INDX": indxLog,
	"MINR": minrLog,
	"PEER": peerLog,
	"RPCS": rpcsLog,
	"SCRP": scrpLog,
	"SRVR": srvrLog,
	"SYNC": syncLog,
	"TXMP": txmpLog,
}

//initlogrotator初始化日志记录旋转器，将日志写入日志文件并在同一目录中创建滚动文件。必须
//在使用包全局日志旋转器变量。
func initLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory:%v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, 10*1024, false, 3)
	if err != nil {
		fmt.Fprint(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}
	logRotator = r
}

//setloglevel为提供的子系统设置日志级别。无效忽略子系统,未初始化的子系统动态创建为需要。
func setLogLevel(subsystemID string, logLevel string) {
	//忽略无效的子系统
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	//如果日志级别无效，则默认为INFO
	level, _ := btcdLog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

//setloglevels将所有子系统记录器的日志级别设置为水平。它还根据需要动态创建子系统记录器，因
//此可用于初始化日志记录系统。
func setLogLevels(logLevel string) {
	//使用新的日志级别配置所有的子系统，动态地
	//根据需要创建记录器
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

//DirectionString是一个助手函数，它返回一个表示连接的方向（入站或出站)
func directionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}

//picknoun返回名词的单复数形式,具体取决于在计数上
func pickNoun(n uint64, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}


// over