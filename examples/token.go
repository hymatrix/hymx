package main

import (
	"math/big"

	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar/schema"
)

func transfer(from *sdk.SDK, to string, amt *big.Int) error {
	info, _ := from.Client.Info()
	_, err := from.SendMessageAndWait(info.Token, "",
		[]schema.Tag{
			{Name: "Action", Value: "Transfer"},
			{Name: "Recipient", Value: to},
			{Name: "Quantity", Value: amt.String()},
		},
	)
	return err
}

func stake(from *sdk.SDK, amt *big.Int) {
	info, _ := from.Client.Info()
	from.SendMessage(info.Token, "",
		[]schema.Tag{
			{Name: "Action", Value: "Stake"},
			{Name: "Quantity", Value: amt.String()},

			{Name: "Registry", Value: info.Registry},
			{Name: "Acc-Id", Value: from.GetAddress()},
			{Name: "Name", Value: "stakeNode"},
			{Name: "Desc", Value: "This is stake node"},
			{Name: "URL", Value: "http://127.0.0.1:8081"},
		},
	)
}
