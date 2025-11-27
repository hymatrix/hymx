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

	DailyLimit int64 `json:"Daily-Limit"`

	DeveloperShareRatio *big.Int `json:"Developer-Share-Ratio"`
}

type X402Response struct {
	X402Version string          `json:"x402Version"`
	Error       string          `json:"error,omitempty"`
	Accepts     []PaymentOption `json:"accepts"`
}

type PaymentOption struct {
	Scheme   string `json:"scheme"`
	Network  string `json:"network"`
	Resource string `json:"resource"`
	PayTo    string `json:"payTo"`
	Asset    string `json:"asset"`
	Amount   string `json:"amount"`
}
