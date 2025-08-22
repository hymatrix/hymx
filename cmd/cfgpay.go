package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/pay"
	"github.com/hymatrix/hymx/pay/schema"
	"github.com/permadao/goar"
	"github.com/spf13/viper"
)

func LoadPayConfig() (p *pay.Pay, err error) {
	if !viper.GetBool("enablePayment") {
		return nil, nil
	}

	url := viper.GetString("payment.URL")
	prvKey := viper.GetString("payment.prvKey")

	signer, err := goether.NewSigner(prvKey)
	if err != nil {
		return
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		return
	}

	settlementAddrStr := viper.GetString("payment.settlementAddress")
	axToken := viper.GetString("payment.axToken")
	txFeeStr := viper.GetString("payment.txFee")
	spawnFeeStr := viper.GetString("payment.spawnFee")
	residencyFeeStr := viper.GetString("payment.residencyFee")
	devRatioStr := viper.GetString("payment.developerShareRatio")

	cfg := &schema.Config{
		SettlementAddress:   common.HexToAddress(settlementAddrStr),
		AxToken:             axToken,
		TxFee:               mustBigInt(txFeeStr),
		SpawnFee:            mustBigInt(spawnFeeStr),
		ResidencyFee:        mustBigInt(residencyFeeStr),
		DeveloperShareRatio: mustBigInt(devRatioStr),
	}

	p = pay.New(url, bundler, cfg)
	return
}

func mustBigInt(s string) *big.Int {
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(fmt.Sprintf("invalid big.Int string: %v", s))
	}
	return bi
}
