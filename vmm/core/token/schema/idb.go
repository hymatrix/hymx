package schema

import "math/big"

const (
	TokenName     = "tHyMatrix"
	TokenTicker   = "tAX"
	TokenLogo     = "4LNO3k5gZbllUlj2N8EoCM99jRe_RvJRdSCKF2xWwwk"
	TokenDecimals = int64(12)
)

var (
	StakeMinAmount = big.NewInt(1000000000000)
)

type Info struct {
	Id       string
	Name     string
	Ticker   string
	Decimals int64
	Logo     string

	MinAmount *big.Int
}

type IDB interface {
	GetInfo() Info
	GetTotalSupply() (*big.Int, error)

	BalanceOf(accId string) (*big.Int, error)
	Balances() (map[string]*big.Int, error)
	UpdateBalance(accId string, amount *big.Int) error

	StakeOf(accId string) (*big.Int, error)
	Stakes() (map[string]*big.Int, error)
	UpdateStake(accId string, amount *big.Int) error

	Checkpoint() (data string, err error)
	Restore(data string) error
}
