package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/everFinance/goether"
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/sdk"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/hymatrix/hymx/vmm/core/token/schema"
	"github.com/permadao/goar"
	"github.com/spf13/viper"
)

var (
	// url = "https://hymatrix.ai"
	url = "http://127.0.0.1:8080"

	prvKey = "0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b"
	// prvKey     = "0xbf9cdeced304f1ef845362c4527fe4ddaecb3fa36fa9ed615447abdffdd16418"
	signer, _  = goether.NewSigner(prvKey)
	bundler, _ = goar.NewBundler(signer)
	s          = sdk.NewFromBundler(url, bundler)
	s2         = sdk.New(url, "./test_keyfile2.json")

	mainNode = registrySchema.Node{
		Name: "test",
		Desc: "test node",
		URL:  url,
	}
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("please input cmd, ex: init, transfer, stake, deposit, module, upload, modules, trysend <pid> <target>, getcache <pid> <key>, nodes, node <accid>, nodesByProcess <pid>, processes <accid>")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		tokenPid, err := initToken()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		initRegistry(tokenPid, mainNode)
	case "token_info":
		tokenInfo(s)
	case "transfer":
		// transfer(s, s2.GetAddress(), schema.StakeMinAmount)
		transfer(s, s2.GetAddress(), big.NewInt(1000))
	case "stake":
		stake(s, schema.StakeMinAmount)
	case "deposit":
		deposit(s, "0xc2835a6caa18CCD33a79C62D104FEA817d715149", "UB0yJx53xBo_rFA4CvKP-WKO25M7kIGrqm2caarghkc", big.NewInt(100000000000))
	case "module_gen":
		genModule()
	case "module_load":
		if len(os.Args) < 3 {
			fmt.Println("please input module id, ex: go run ./ module_load <id>")
			os.Exit(1)
		}
		loadModule(os.Args[2])
	case "module_upload":
		if len(os.Args) < 3 {
			fmt.Println("please input module file, ex: go run ./ module_upload <modfile>")
			os.Exit(1)
		}
		modFile := os.Args[2]
		txid, err := uploadModule(modFile, "./test_keyfile.json")
		if err != nil {
			fmt.Println("upload module failed, ", "err", err)
			return
		}
		fmt.Println("upload module success, ", "txid", txid)
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
	case "modules":
		modulesCmd()
	case "nodes":
		nodesCmd()
	case "node":
		if len(os.Args) < 3 {
			fmt.Println("usage: node <accid>")
			os.Exit(1)
		}
		nodeCmd(os.Args[2])
	case "nodesByProcess":
		if len(os.Args) < 3 {
			fmt.Println("usage: nodesByProcess <pid>")
			os.Exit(1)
		}
		nodesByProcessCmd(os.Args[2])
	case "processes":
		if len(os.Args) < 3 {
			fmt.Println("usage: processes <accid>")
			os.Exit(1)
		}
		processesCmd(os.Args[2])
	case "trysend":
		if len(os.Args) < 4 {
			fmt.Println("usage: trysend <pid> <target>")
			os.Exit(1)
		}
		pid := os.Args[2]
		target := os.Args[3]
		trySendCmd(pid, target)
	case "getcache":
		if len(os.Args) < 4 {
			fmt.Println("usage: getcache <pid> <key>")
			os.Exit(1)
		}
		pid := os.Args[2]
		key := os.Args[3]
		getCacheCmd(pid, key)
	default:
		fmt.Printf("unknown cmd: %s\n", cmd)
		os.Exit(1)
	}
}
