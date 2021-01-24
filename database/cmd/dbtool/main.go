package main

import (
	"BtcoinProject/database"
	"github.com/btcsuite/btclog"
	"github.com/jessevdk/go-flags"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (

	//blockdbnameprefix is the prefix for the btcd block database
	blockDbNamePrefix = "blocks"
)

var (
	log             btclog.Logger
	shutdownChannel = make(chan error)
)

//realmain is the real main function for the utility .it is necessary to work
//around the fact that defered functions do not run when os.exist()is caller.
func realMain() error {
	// Setup logging.
	backendLogger := btclog.NewBackend(os.Stdout)
	defer os.Stdout.Sync()
	log = backendLogger.Logger("MAIN")
	dbLog := backendLogger.Logger("BCDB")
	dbLog.SetLevel(btclog.LevelDebug)
	database.UseLogger(dbLog)

	// Setup the parser options and commands.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	parserFlags := flags.Options(flags.HelpFlag | flags.PassDoubleDash)
	parser := flags.NewNamedParser(appName, parserFlags)
	parser.AddGroup("Global Options", "", cfg)
	parser.AddCommand("insecureimport",
		"Insecurely import bulk block data from bootstrap.dat",
		"Insecurely import bulk block data from bootstrap.dat.  "+
			"WARNING: This is NOT secure because it does NOT "+
			"verify chain rules.  It is only provided for testing "+
			"purposes.", &importCfg)
	parser.AddCommand("loadheaders",
		"Time how long to load headers for all blocks in the database",
		"", &headersCfg)
	parser.AddCommand("fetchblock",
		"Fetch the specific block hash from the database", "",
		&fetchBlockCfg)
	parser.AddCommand("fetchblockregion",
		"Fetch the specified block region from the database", "",
		&blockRegionCfg)

	// Parse command line and invoke the Execute function for the specified
	// command.
	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		} else {
			log.Error(err)
		}

		return err
	}

	return nil

}

//loadblockdb opens the block database and reutrns a handle to it
func loadBlockDB() (database.DB, error) {

	//the database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + cfg.DbType
	dbPath := filepath.Join(cfg.DataDir, dbName)

	log.Infof("loading block database from '%s'", dbPath)
	db, err := database.Open(cfg.DbType, dbPath, activeNetParams.Net)
	if err != nil {
		//return the error if it is not because the database do inn,t
		//eixst
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {
			return nil, err
		}

		//create the  db if it does not exist
		err = os.Mkdir(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}

		db, err = database.Create(cfg.DbType, dbPath, activeNetParams.Net)
		if err != nil {
			return nil, err
		}

	}

	log.Info("block database loaded")
	return db, nil

}

func main() {
	//use all processer cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	//work around defer not working after os.exit()
	if err := realMain(); err != nil {
		os.Exit(1)
	}

}

//over