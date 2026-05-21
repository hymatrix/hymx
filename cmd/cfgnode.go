package main

import (
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/cryptor"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
	"github.com/spf13/viper"
)

func LoadNodeConfig() (
	port, adminPort, ginMode, redisURL, arweaveURL, hymxURL string,
	bundler *goar.Bundler, nodeInfo *nodeSchema.Info, decryptor *cryptor.Cryptor, err error,
) {
	port = viper.GetString("port")
	adminPort = viper.GetString("adminPort")
	ginMode = viper.GetString("ginMode")
	redisURL = viper.GetString("redisURL")
	arweaveURL = viper.GetString("arweaveURL")
	hymxURL = viper.GetString("hymxURL")
	prvKey := viper.GetString("prvKey")
	keyfilePath := viper.GetString("keyfilePath")

	var signer interface{}
	if prvKey != "" {
		signer, err = goether.NewSigner(prvKey)
		if err == nil {
			decryptor, err = cryptor.NewECCFromPrivateKey(prvKey)
		}
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

	var pubkey string
	if decryptor != nil {
		pubkey, err = decryptor.PublicKey()
		if err != nil {
			return
		}
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
		EncryptionPublicKey: pubkey,
	}

	return
}
