package main

import (
	"math/big"

	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar/schema"
)

func deposit(from *sdk.SDK, to, beneficiary string, amt *big.Int) error {
	info, _ := from.Client.Info()
	_, err := from.SendMessageAndWait(info.Token, "",
		[]schema.Tag{
			{Name: "Action", Value: "Transfer"},
			{Name: "Recipient", Value: to},
			{Name: "Quantity", Value: amt.String()},
			{Name: "Beneficiary", Value: beneficiary},
		},
	)
	return err
}
