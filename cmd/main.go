package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/everFinance/goether"
	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/server"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/inconshreveable/log15"
	"github.com/permadao/goar"
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

	// config
	port := viper.GetString("port")
	ginMode := viper.GetString("ginMode")
	redisURL := viper.GetString("redisURL")
	arweaveURL := viper.GetString("arweaveURL")
	hymxURL := viper.GetString("hymxURL")
	prvKey := viper.GetString("prvKey")
	keyfilePath := viper.GetString("keyfilePath")

	gin.SetMode(ginMode)
	if ginMode == "release" {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler))

	}

	var signer interface{}
	if prvKey != "" {
		signer, err = goether.NewSigner(prvKey)
	} else {
		signer, err = goar.NewSignerFromPath(keyfilePath)
	}
	if err != nil {
		return err
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		return err
	}

	// config
	nodeInfo := &nodeSchema.Info{
		Protocol:    schema.DataProtocol,
		Variant:     schema.Variant,
		NodeVersion: nodeSchema.NodeVersion,
		JoinNetwork: viper.GetBool("joinNetwork"),
		Node: registrySchema.Node{
			AccId: bundler.Address,
			Name:  viper.GetString("nodeName"),
			Desc:  viper.GetString("nodeDesc"),
			URL:   viper.GetString("nodeURL"),
		},
	}

	s := server.New(bundler, redisURL, arweaveURL, hymxURL, nodeInfo)

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
