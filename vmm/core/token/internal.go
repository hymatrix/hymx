package token

import (
	"math/big"

	"github.com/hymatrix/hymx/vmm/core/token/schema"
	"github.com/hymatrix/hymx/vmm/utils"
)

func (v *Token) transfer(from, to string, amount *big.Int) (err error) {
	_, to, err = utils.IDCheck(to)
	if err != nil {
		return
	}

	if err = v.sub(from, amount); err != nil {
		log.Error("transfer: token sub failed", "err", err)
		return
	}
	if err = v.add(to, amount); err != nil {
		log.Error("transfer: token add failed", "err", err)
		return
	}
	return nil
}

func (v *Token) stake(from string, amount *big.Int) (err error) {
	if err = v.sub(from, amount); err != nil {
		log.Error("stake: token sub failed", "err", err)
		return
	}
	if err = v.sadd(from, amount); err != nil {
		log.Error("stake: token add failed", "err", err)
		return
	}
	return nil
}

func (v *Token) unstake(from string, amount *big.Int) (err error) {
	if err = v.ssub(from, amount); err != nil {
		log.Error("unstake: token sub failed", "err", err)
		return
	}
	if err = v.add(from, amount); err != nil {
		log.Error("unstake: token add failed", "err", err)
		return
	}
	return nil
}

func (v *Token) slash(from string, amount *big.Int) (err error) {
	// TODO
	return nil
}

/////////////////////////////////////////////////////////////////

func (v *Token) add(accId string, amount *big.Int) error {
	// if amount == 0, then return nil
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	bal, err := v.db.BalanceOf(accId)
	if err != nil {
		return err
	}

	return v.db.UpdateBalance(accId, new(big.Int).Add(bal, amount))
}

func (v *Token) sub(accId string, amount *big.Int) error {
	// if amount == 0, then return nil
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	bal, err := v.db.BalanceOf(accId)
	if err != nil {
		return err
	}

	if bal.Cmp(amount) < 0 {
		return schema.ErrInsufficientBalance
	}

	return v.db.UpdateBalance(accId, new(big.Int).Sub(bal, amount))
}

func (v *Token) sadd(accId string, amount *big.Int) error {
	// if amount == 0, then return nil
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	bal, err := v.db.StakeOf(accId)
	if err != nil {
		return err
	}

	return v.db.UpdateStake(accId, new(big.Int).Add(bal, amount))
}

func (v *Token) ssub(accId string, amount *big.Int) error {
	// if amount == 0, then return nil
	if amount.Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	bal, err := v.db.StakeOf(accId)
	if err != nil {
		return err
	}

	if bal.Cmp(amount) < 0 {
		return schema.ErrInsufficientStake
	}

	return v.db.UpdateStake(accId, new(big.Int).Sub(bal, amount))
}
