package main

import (
	"fmt"
	"os"

	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/vmm/core/token/schema"
	"github.com/permadao/goar"
)

var (
	// url = "http://127.0.0.1:8080"
	url = "https://hymatrix.ai"

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
		transfer(s, s2.GetAddress(), schema.StakeMinAmount)
	case "stake":
		stake(s, schema.StakeMinAmount)
	case "module":
		module()
	default:
		fmt.Printf("unknown cmd: %s\n", cmd)
		os.Exit(1)
	}
}
