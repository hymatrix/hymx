package main

import (
	"fmt"
	"strconv"

	"github.com/hymatrix/hymx/chainkit"
	chainkitSchema "github.com/hymatrix/hymx/chainkit/schema"
	"github.com/hymatrix/hymx/db/rdb"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type NodeDB struct {
	db nodeSchema.IDB
}

func NewNodeDB(redisUrl string) *NodeDB {
	return &NodeDB{
		db: rdb.New(redisUrl),
	}
}

func (n *NodeDB) GetMessage(msgid string) (msg *goarSchema.BundleItem, err error) {
	return n.db.GetMessage(msgid)
}

func (n *NodeDB) GetAssignByMessage(msgid string) (assign *goarSchema.BundleItem, err error) {
	res, err := n.db.GetResult(msgid)
	if err != nil {
		return
	}
	if res == nil {
		return
	}
	nonce, err := strconv.ParseInt(res.Nonce, 10, 64)
	if err != nil {
		return
	}
	return n.db.GetAssignByNonce(res.FromProcess, nonce)
}

func (n *NodeDB) GetAssignByNonce(pid string, nonce int64) (assign *goarSchema.BundleItem, err error) {
	return n.db.GetAssignByNonce(pid, nonce)
}

func Upload(pid string, nodeRedis, chainkitRedis, keyfile string) error {
	node := NewNodeDB(nodeRedis)
	conf := chainkitSchema.Config{
		RedisUrl: chainkitRedis,
		OptType:  "goar",
		Keyfile:  keyfile,
	}
	chainkit := chainkit.New(node, conf)
	chainkit.Run()

	from := int64(0)
	to, err := node.db.GetNonce(pid)
	if err != nil {
		fmt.Printf("GetNonce failed, pid: %s, err: %v\n", pid, err)
		return err
	}

	for nonce := from; nonce <= to; nonce++ {
		tx, err := node.db.GetMessageByNonce(pid, nonce)
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
