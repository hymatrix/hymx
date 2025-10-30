package main

import (
	"fmt"

	"github.com/hymatrix/hymx/chainkit"
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/db/rdb"
)

func Upload(pid string, conf chainkitSchema.Config) error {

	nodeDB := rdb.New(conf.NodeRedisUrl)

	chainkit := chainkit.New(conf)
	chainkit.Run()

	from := int64(0)
	to, err := nodeDB.GetNonce(pid)
	if err != nil {
		fmt.Printf("GetNonce failed, pid: %s, err: %v\n", pid, err)
		return err
	}

	for nonce := from; nonce <= to; nonce++ {
		tx, err := nodeDB.GetMessageByNonce(pid, nonce)
		if err != nil || tx == nil {
			fmt.Printf("GetMessageByNonce failed, nonce: %d, err: %v\n", nonce, err)
			continue
		}
		chainkit.Upload(*tx)
		fmt.Printf("Upload, nonce: %d, txid: %s\n", nonce, tx.Id)
	}

	fmt.Println("Press ENTER to exit...")
	for {
		var input string
		fmt.Scanln(&input)
		if input == " " || len(input) == 0 {
			fmt.Println("Exiting...")
			break
		}
	}
	chainkit.Close()

	return nil
}
