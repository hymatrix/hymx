package schema

import (
	"math/big"
)

type Config struct {
	// ChargeAddress is the address where prepaid fees should be transferred
	ChargeAddress string `json:"Charge-Address"`

	// SettlementAddress is the node’s settlement address (e.g. cold wallet or revenue sink)
	SettlementAddress string `json:"-"`

	AxToken      string   `json:"Ax-Token"`
	TxFee        *big.Int `json:"Tx-Fee"`
	SpawnFee     *big.Int `json:"Spawn-Fee"`
	ResidencyFee *big.Int `json:"Residency-Fee"`

	DeveloperShareRatio *big.Int `json:"Developer-Share-Ratio"`
}
