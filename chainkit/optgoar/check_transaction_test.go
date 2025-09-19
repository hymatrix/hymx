package optgoar

import (
	"context"
	"fmt"
	"testing"

	"github.com/permadao/goar"
)

func TestCheckTransactionWithSpecificTxid(t *testing.T) {
	// 使用测试keyfile初始化wallet
	keyfilePath := "arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	wallet, err := goar.NewWalletFromPath(keyfilePath, "https://arweave.net")
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// 创建OptGoar实例
	ctx := context.Background()
	optGoar := New(wallet, ctx)

	// 测试指定的txid
	txid := "tiB55vvqzNvUOE5AVf5OsaO88R-5rmRHmasinXn3MKE"
	fmt.Printf("Testing CheckTransaction with txid: %s\n", txid)

	// 调用CheckTransaction函数
	isConfirmed, err := optGoar.CheckTransaction(txid)
	if err != nil {
		fmt.Printf("Error checking transaction: %v\n", err)
		t.Logf("Error checking transaction: %v", err)
		return
	}

	fmt.Printf("Transaction confirmed: %t\n", isConfirmed)
	t.Logf("Transaction confirmed: %t", isConfirmed)
}