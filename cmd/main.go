package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/server"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

var log = common.NewLog("hymx-cmd")

func main() {
	cli.VersionFlag = flagVersion

	app := &cli.App{
		Name:     schema.DataProtocol,
		Version:  nodeSchema.NodeVersion,
		Flags:    flags,
		Commands: cmds,
		Action:   action,
	}

	if err := app.Run(os.Args); err != nil {
		log.Error("run server failed", "err", err)
	}
}

func action(c *cli.Context) error {
	// viper configuration
	// notice: viper only for yaml file, cmd flags use urfave
	configPath := c.String("config")
	if configPath == "" {
		configPath = DefaultConfig
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return run(c)
}

func run(c *cli.Context) (err error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// node config
	port, ginMode, redisURL, arweaveURL, hymxURL, bundler, nodeInfo, err := LoadNodeConfig()
	if err != nil {
		return err
	}

	gin.SetMode(ginMode)
	if ginMode == "release" {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler))
	}

	// pay config
	pay, err := LoadPayConfig()
	if err != nil {
		return err
	}

	chainkit, err := LoadChainkitConfig()
	if err != nil {
		return err
	}

	node := node.New(bundler, redisURL, arweaveURL, hymxURL, nodeInfo, chainkit)

	s := server.New(node, pay)

	// mount your vm here.....
	// ex:
	// s.Mount("<moduleFormat>", FuncForSpawn)
	// - moduleFormat: the module format of your vm. The Module-Format tag in your Module
	// - FuncForSpawn: the function for spawn your vm

	// add result handler
	// ex:
	// s.AddResultHandler(handlers)

	s.Run(port)

	log.Info("server is running", "protocol version", schema.Variant, "node version", nodeSchema.NodeVersion, "wallet", bundler.Address, "port", port)

	<-signals
	s.Close()

	return nil
}
