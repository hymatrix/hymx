package vmm

import (
	"math/big"

	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/vmm/core/token"
	tokenSchema "github.com/hymatrix/hymx/vmm/core/token/schema"
	"github.com/hymatrix/hymx/vmm/schema"
)

func (v *Vmm) TokenId() string {
	v.coreMu.RLock()
	defer v.coreMu.RUnlock()
	if v.token == nil {
		return ""
	}
	return v.token.Info().Id
}

func (v *Vmm) BalanceOf(accid string) (*big.Int, error) {
	v.coreMu.RLock()
	defer v.coreMu.RUnlock()
	if v.token == nil {
		return nil, nil
	}
	return v.token.BalanceOf(accid)
}

func (v *Vmm) StakeOf(accid string) (*big.Int, error) {
	v.coreMu.RLock()
	defer v.coreMu.RUnlock()
	if v.token == nil {
		return nil, nil
	}
	return v.token.StakeOf(accid)
}

func (v *Vmm) spawnToken(env schema.Env) (vm schema.Vm, err error) {
	v.coreMu.Lock()
	defer v.coreMu.Unlock()
	if v.token != nil {
		return nil, schema.ErrTokenAlreadyCreated
	}

	genesisBalance := new(big.Int)
	genesisBalance.SetString("20000000000000000000", 10)
	genesisStake := new(big.Int)
	genesisStake.SetString("1000000000000000000", 10)

	db := cache.NewToken(
		tokenSchema.Info{
			Id:        env.Meta.Pid,
			Name:      tokenSchema.TokenName,
			Ticker:    tokenSchema.TokenTicker,
			Decimals:  tokenSchema.TokenDecimals,
			Logo:      tokenSchema.TokenLogo,
			MinAmount: tokenSchema.StakeMinAmount,
		},
		map[string]*big.Int{
			env.Meta.AccId: genesisBalance,
		},
		map[string]*big.Int{
			env.Meta.AccId: genesisStake,
		},
	)
	tkvm, err := token.New(db)
	if err != nil {
		return
	}
	v.token = tkvm

	return tkvm, nil
}
