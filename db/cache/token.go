package cache

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/hymatrix/hymx/db/cache/schema"
	tokenSchema "github.com/hymatrix/hymx/vmm/core/token/schema"
)

type Token struct {
	info tokenSchema.Info

	totalSupply *big.Int
	balances    map[string]*big.Int
	stakes      map[string]*big.Int

	initialCache bool
	rwlock       sync.RWMutex
}

func NewToken(info tokenSchema.Info, bals, stakes map[string]*big.Int) *Token {
	total := big.NewInt(0)
	for _, b := range bals {
		total = total.Add(total, b)
	}

	return &Token{
		info: info,

		totalSupply:  total,
		balances:     bals,
		stakes:       stakes,
		initialCache: false,
	}
}

func (h *Token) GetInfo() tokenSchema.Info {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()
	return h.info
}

func (h *Token) GetTotalSupply() (*big.Int, error) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	copy := new(big.Int).Set(h.totalSupply)
	return copy, nil
}

func (h *Token) BalanceOf(accId string) (*big.Int, error) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	if bal, ok := h.balances[accId]; ok {
		return new(big.Int).Set(bal), nil
	}

	return big.NewInt(0), nil
}

func (h *Token) Balances() (map[string]*big.Int, error) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	copy := make(map[string]*big.Int, len(h.balances))
	for k, v := range h.balances {
		copy[k] = new(big.Int).Set(v)
	}
	return copy, nil
}

func (h *Token) UpdateBalance(accId string, amount *big.Int) error {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	if amount == nil {
		return fmt.Errorf("amount is nil")
	}
	h.balances[accId] = new(big.Int).Set(amount)

	return nil
}

func (h *Token) StakeOf(accId string) (*big.Int, error) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	if bal, ok := h.stakes[accId]; ok {
		return new(big.Int).Set(bal), nil
	}

	return big.NewInt(0), nil
}

func (h *Token) Stakes() (map[string]*big.Int, error) {
	h.rwlock.RLock()
	defer h.rwlock.RUnlock()

	copy := make(map[string]*big.Int, len(h.stakes))
	for k, v := range h.stakes {
		copy[k] = new(big.Int).Set(v)
	}
	return copy, nil
}

func (h *Token) UpdateStake(accId string, amount *big.Int) error {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	if amount == nil {
		return fmt.Errorf("amount is nil")
	}
	h.stakes[accId] = new(big.Int).Set(amount)

	return nil
}

func (h *Token) CacheInitial() bool {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()
	return h.initialCache
}

func (h *Token) CacheInitialed() {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()
	h.initialCache = true
	return
}

func (h *Token) Checkpoint() (data string, err error) {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	sp := schema.TokenSnapshot{
		Id:        h.info.Id,
		Name:      h.info.Name,
		Ticker:    h.info.Ticker,
		Decimals:  h.info.Decimals,
		Logo:      h.info.Logo,
		MinAmount: h.info.MinAmount,

		TotalSupply:  h.totalSupply,
		Balances:     h.balances,
		Stakes:       h.stakes,
		InitialCache: h.initialCache,
	}

	by, err := json.Marshal(sp)
	if err != nil {
		return
	}
	data = string(by)

	return
}

func (h *Token) Restore(data string) error {
	h.rwlock.Lock()
	defer h.rwlock.Unlock()

	sp := &schema.TokenSnapshot{}
	if err := json.Unmarshal([]byte(data), sp); err != nil {
		return err
	}

	h.totalSupply = sp.TotalSupply
	h.balances = sp.Balances
	h.stakes = sp.Stakes
	h.initialCache = sp.InitialCache

	info := tokenSchema.Info{
		Id:        sp.Id,
		Name:      sp.Name,
		Ticker:    sp.Ticker,
		Decimals:  sp.Decimals,
		Logo:      sp.Logo,
		MinAmount: sp.MinAmount,
	}

	h.info = info
	return nil
}
