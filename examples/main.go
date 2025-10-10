package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/everFinance/goether"
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/vmm/core/token/schema"
	"github.com/permadao/goar"
	"github.com/spf13/viper"
)

var (
	// url = "https://hymatrix.ai"
	url = "http://127.0.0.1:8080"

	prvKey     = "0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b"
	signer, _  = goether.NewSigner(prvKey)
	bundler, _ = goar.NewBundler(signer)
	s          = sdk.NewFromBundler(url, bundler)
	s2         = sdk.New(url, "./test_keyfile2.json")
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("please input cmd, ex: pingpong, sendMessage, spawn, eval, eval2, receive, receive2, reply, inbox, result, checkpoint, ollama, recover1, recover2")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		initRegistry(initToken())
	case "transfer":
		// transfer(s, s2.GetAddress(), schema.StakeMinAmount)
		transfer(s, s2.GetAddress(), big.NewInt(1000))
	case "stake":
		stake(s, schema.StakeMinAmount)
	case "deposit":
		deposit(s, "0xc2835a6caa18CCD33a79C62D104FEA817d715149", "UB0yJx53xBo_rFA4CvKP-WKO25M7kIGrqm2caarghkc", big.NewInt(100000000000))
	case "module":
		module()
	case "upload":
		if len(os.Args) < 3 {
			fmt.Println("please input pid, ex: go run ./ upload <pid>")
			os.Exit(1)
		}
		pid := os.Args[2]
		configPath := "./config_chainkit.yaml"
		viper.SetConfigFile(configPath)
		viper.SetConfigType("yaml")
		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("read config file %s failed, err: %v\n", configPath, err)
			os.Exit(1)
		}
		cfg := chainkitSchema.Config{
			RedisUrl:     viper.GetString("chainkit.redisURL"),
			NodeRedisUrl: viper.GetString("chainkit.nodeRedisURL"),
			Keyfile:      viper.GetString("chainkit.keyfilePath"),
			OptType:      viper.GetString("chainkit.optType"),
		}
		Upload(pid, cfg)
	default:
		fmt.Printf("unknown cmd: %s\n", cmd)
		os.Exit(1)
	}
}
