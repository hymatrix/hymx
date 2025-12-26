package token

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/big"

	"github.com/hymatrix/hymx/vmm/utils"
)

func (h *Token) initCache() (cache map[string]string) {
	if h.db.CacheInitial() {
		return
	}
	defer h.db.CacheInitialed()

	balances, err := h.db.Balances()
	if err != nil {
		return
	}
	cacheBalanceMap := make(map[string]string)
	for k, vl := range balances {
		if vl == nil {
			vl = big.NewInt(0)
		}
		cacheBalanceMap["balances:"+k] = vl.String()
	}

	stakes, err := h.db.Stakes()
	if err != nil {
		return
	}
	cacheStakeMap := make(map[string]string)
	for k, vl := range stakes {
		if vl == nil {
			vl = big.NewInt(0)
		}
		cacheStakeMap["stakes:"+k] = vl.String()
	}

	cache = map[string]string{}
	maps.Copy(cache, cacheBalanceMap)
	maps.Copy(cache, cacheStakeMap)
	maps.Copy(cache, h.cacheTotalSupply())
	maps.Copy(cache, h.cacheTokenInfo())
	return
}

func (h *Token) cacheTokenInfo() map[string]string {
	info := h.Info()
	tokenInfo := map[string]string{
		"Name":         info.Name,
		"Ticker":       info.Ticker,
		"Logo":         info.Logo,
		"Denomination": fmt.Sprintf("%d", info.Decimals),
		"Decimals":     fmt.Sprintf("%d", info.Decimals),
	}
	res, _ := json.Marshal(tokenInfo)
	return map[string]string{
		"info": string(res),
	}
}

func (h *Token) cacheChangeBalance(updateAccounts ...string) map[string]string {
	cacheMap := make(map[string]string)
	for _, acc := range updateAccounts {
		_, accid, _ := utils.IDCheck(acc)
		bal, err := h.db.BalanceOf(accid)
		if err != nil {
			bal = big.NewInt(0)
		}
		cacheMap["balances:"+accid] = bal.String()
	}
	return cacheMap
}

func (h *Token) cacheTotalSupply() map[string]string {
	cacheMap := make(map[string]string)
	totalSupply, err := h.db.GetTotalSupply()
	if err == nil {
		cacheMap["total-supply"] = totalSupply.String()
	}
	return cacheMap
}

func (h *Token) cacheChangeStake(accId string) map[string]string {
	cacheMap := make(map[string]string)
	stake, err := h.db.StakeOf(accId)
	if err != nil {
		stake = big.NewInt(0)
	}
	cacheMap["stakes:"+accId] = stake.String()
	return cacheMap
}
