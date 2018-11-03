package main

import (
	"flag"
	"fmt"

	"github.com/dynamicgo/config"
	"github.com/dynamicgo/slf4go"
	"github.com/go-xorm/xorm"
	"github.com/inwecrypto/tokenswap"
	_ "github.com/lib/pq"
)

var logger = slf4go.Get("tokenswap")
var configpath = flag.String("conf", "./tokenswap.json", "geth indexer config file path")

func main() {

	flag.Parse()

	conf, err := config.NewFromFile(*configpath)

	if err != nil {
		logger.ErrorF("load eth indexer config err , %s", err)
		return
	}

	name := "tokenswapdb"

	driver := conf.GetString(fmt.Sprintf("%s.driver", name), "postgres")
	datasource := conf.GetString(fmt.Sprintf("%s.datasource", name), "")

	engine, err := xorm.NewEngine(driver, datasource)

	if err != nil {
		logger.ErrorF("create postgres orm engine err , %s", err)
		return
	}

	if err := engine.Sync2(new(tokenswap.Order), new(tokenswap.Log), new(tokenswap.SendOrder)); err != nil {
		logger.ErrorF("sync table schema error , %s", err)
		return
	}

}
