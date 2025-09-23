package main

import (
	"github.com/everFinance/goether"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
	"github.com/spf13/viper"
)

func LoadNodeConfig() (
	port, ginMode, redisURL, arweaveURL, hymxURL string,
	bundler *goar.Bundler, nodeInfo *nodeSchema.Info, err error,
) {
	port = viper.GetString("port")
	ginMode = viper.GetString("ginMode")
	redisURL = viper.GetString("redisURL")
	arweaveURL = viper.GetString("arweaveURL")
	hymxURL = viper.GetString("hymxURL")
	prvKey := viper.GetString("prvKey")
	keyfilePath := viper.GetString("keyfilePath")

	var signer interface{}
	if prvKey != "" {
		signer, err = goether.NewSigner(prvKey)
	} else {
		signer, err = goar.NewSignerFromPath(keyfilePath)
	}
	if err != nil {
		return
	}
	bundler, err = goar.NewBundler(signer)
	if err != nil {
		return
	}

	nodeInfo = &nodeSchema.Info{
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

	return
}
