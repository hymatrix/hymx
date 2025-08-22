package pay

import (
	"math/big"

	"github.com/hymatrix/hymx/pay/schema"
)

func (p *Pay) handleDeposit(txHash, token, sponsor, beneficiary string, qty *big.Int) error {
	if p.db.IsExecuted(txHash) {
		return schema.ErrTxAlreadyExecuted
	}

	p.db.MarkExecuted(txHash)
	return p.db.Deposit(sponsor, beneficiary, qty)
}

func (p *Pay) handleWithdraw(token, sponsor, beneficiary string, qty *big.Int) {}
