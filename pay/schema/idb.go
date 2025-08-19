package schema

import "math/big"

type IDB interface {
	IsWhitelist()
	BalanceOf(tokenId, accId string) (*big.Int, error)
	UpdateBalance(tokenId, accId string, amount *big.Int) error
}
