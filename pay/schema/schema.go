package schema

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	// SettlementAddress is the node’s settlement address (e.g. cold wallet or revenue sink)
	SettlementAddress common.Address

	AxToken      string
	TxFee        *big.Int
	SpawnFee     *big.Int
	ResidencyFee *big.Int

	DeveloperShareRatio *big.Int
}
