package main

import (
	"flag"

	"github.com/dynamicgo/aliyunlog"
	"github.com/dynamicgo/config"
	"github.com/dynamicgo/slf4go"
	kafka "github.com/inwecrypto/gomq-kafka"
	"github.com/inwecrypto/tokenswap"
	_ "github.com/lib/pq"
)

var logger = slf4go.Get("tokenswap")
var configpath = flag.String("conf", "./tokenswap.json", "tokenswap config json")

var levels = map[string]int{
	"debug": slf4go.Fatal | slf4go.Error | slf4go.Warn | slf4go.Info | slf4go.Debug,
	"info":  slf4go.Fatal | slf4go.Error | slf4go.Warn | slf4go.Info,
	"warn":  slf4go.Fatal | slf4go.Error | slf4go.Warn,
	"error": slf4go.Fatal | slf4go.Error,
	"fatal": slf4go.Fatal,
}

func main() {

	flag.Parse()

	conf, err := config.NewFromFile(*configpath)

	if err != nil {
		logger.ErrorF("load tokenswap config err , %s", err)
		return
	}

	if conf.GetString("slf4go.backend", "") == "aliyun" {
		factory, err := aliyunlog.NewAliyunBackend(conf)

		if err != nil {
			logger.ErrorF("create aliyun log backend err , %s", err)
			return
		}

		slf4go.Backend(factory)
	}

	loglevel, ok := levels[conf.GetString("slf4go.level", "debug")]

	if !ok {
		loglevel = levels["debug"]
	}

	slf4go.SetLevel(loglevel)

	neoconf := conf.GetConfig("neo")

	neomq, err := kafka.NewAliyunConsumer(neoconf)

	if err != nil {
		logger.ErrorF("create neo kafka mq err , %s", err)
		return
	}

	ethconf := conf.GetConfig("eth")

	ethmq, err := kafka.NewAliyunConsumer(ethconf)

	if err != nil {
		logger.ErrorF("create eth kafka mq err , %s", err)
		return
	}

	monitor, err := tokenswap.NewMonitor(conf, neomq, ethmq)

	if err != nil {
		logger.ErrorF("create monitor err , %s", err)
		return
	}

	monitor.Run()

	web, err := tokenswap.NewWebServer(conf)
	if err != nil {
		logger.ErrorF("create web server err , %s", err)
		return
	}

	logger.InfoF("web server started.")
	web.Run()

}
