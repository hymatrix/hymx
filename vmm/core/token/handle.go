package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"strings"

	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	registryUtils "github.com/hymatrix/hymx/vmm/core/registry/utils"
	"github.com/hymatrix/hymx/vmm/core/token/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (h *Token) handleCacheInitial() (res vmmSchema.Result, err error) {
	if !h.db.CacheInitial() {
		err = errors.New("cache_initialed")
		return
	}

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

	res.Cache = map[string]string{}
	maps.Copy(res.Cache, cacheBalanceMap)
	maps.Copy(res.Cache, cacheStakeMap)
	maps.Copy(res.Cache, h.cacheTotalSupply())
	maps.Copy(res.Cache, h.cacheTokenInfo())

	h.db.CacheInitialed()
	return
}

func (h *Token) handleInfo(from string) (res vmmSchema.Result, err error) {
	info := h.Info()
	decimals := fmt.Sprintf("%v", info.Decimals)

	res.Messages = []*vmmSchema.ResMessage{
		{
			Target: from,
			Tags: []goarSchema.Tag{
				{Name: "Name", Value: info.Name},
				{Name: "Ticker", Value: info.Ticker},
				{Name: "Logo", Value: info.Logo},
				{Name: "Denomination", Value: decimals},
				{Name: "Decimals", Value: decimals},
			},
		},
	}
	return
}

func (h *Token) handleTotalSupply(from string) (res vmmSchema.Result, err error) {
	totalSupply, err := h.db.GetTotalSupply()
	if err != nil {
		return
	}

	res.Messages = []*vmmSchema.ResMessage{
		{
			Target: from,
			Data:   totalSupply.String(),
			Tags: []goarSchema.Tag{
				{Name: "Action", Value: "Total-Supply"},
				{Name: "Ticker", Value: h.Info().Ticker},
			},
		},
	}
	return
}

func (h *Token) handleBalanceOf(from string, params map[string]string) (res vmmSchema.Result, err error) {
	accid := from
	if recipient, ok := params["Recipient"]; ok {
		accid = recipient
	} else if target, ok := params["Target"]; ok {
		accid = target
	}
	bal, err := h.BalanceOf(accid)
	if err != nil {
		return
	}

	res.Messages = []*vmmSchema.ResMessage{
		{
			Target: from,
			Data:   bal.String(),
			Tags: []goarSchema.Tag{
				{Name: "Balance", Value: bal.String()},
				{Name: "Ticker", Value: h.Info().Ticker},
				{Name: "Account", Value: accid},
			},
		},
	}
	return
}

func (h *Token) handleBalances(from string) (res vmmSchema.Result, err error) {
	balances := make(map[string]string)
	bals, err := h.db.Balances()
	if err != nil {
		return
	}
	for k, v := range bals {
		balances[k] = v.String()
	}

	balancesJs, err := json.Marshal(balances)
	if err != nil {
		return
	}
	res.Messages = []*vmmSchema.ResMessage{
		{
			Target: from,
			Data:   string(balancesJs),
			Tags: []goarSchema.Tag{
				{Name: "Ticker", Value: h.Info().Ticker},
			},
		},
	}
	return
}

func (h *Token) handleTransfer(from string, params map[string]string) (res vmmSchema.Result, err error) {
	recipient, ok := params["Recipient"]
	if !ok {
		err = schema.ErrMissingRecipient
		return
	}
	qty, ok := params["Quantity"]
	if !ok {
		err = schema.ErrMissingQuantity
		return
	}
	amt, err := h.parseAndCheckAmount(qty)
	if err != nil {
		return
	}

	if err = h.transfer(from, recipient, amt); err != nil {
		return
	}

	debitNotice := &vmmSchema.ResMessage{
		Target: from,
		Data:   "You transferred " + qty + " to " + recipient,
		Tags: []goarSchema.Tag{
			{Name: "Ticker", Value: h.Info().Ticker},
			{Name: "Action", Value: "Debit-Notice"},
			{Name: "Recipient", Value: recipient},
			{Name: "Quantity", Value: qty},
		},
	}
	creditNotice := &vmmSchema.ResMessage{
		Target: recipient,
		Data:   "You received " + qty + " from " + from,
		Tags: []goarSchema.Tag{
			{Name: "Ticker", Value: h.Info().Ticker},
			{Name: "Action", Value: "Credit-Notice"},
			{Name: "Sender", Value: from},
			{Name: "Quantity", Value: qty},
		},
	}
	// Forward X- prefixed tags to both messages
	for key, value := range params {
		if strings.HasPrefix(key, "X-") {
			debitNotice.Tags = append(debitNotice.Tags, goarSchema.Tag{Name: key, Value: value})
			creditNotice.Tags = append(creditNotice.Tags, goarSchema.Tag{Name: key, Value: value})
		}
	}

	res.Messages = []*vmmSchema.ResMessage{debitNotice, creditNotice}
	res.Cache = h.cacheChangeBalance(from, recipient)
	return
}

func (h *Token) handleStake(from string, params map[string]string) (res vmmSchema.Result, err error) {
	registry, ok := params["Registry"]
	if !ok {
		err = schema.ErrMissingQuantity
		return
	}
	node, err := registryUtils.Decode(params)
	if err != nil {
		return
	}
	if from != node.AccId {
		err = schema.ErrUnauthorized
		return
	}

	qty, ok := params["Quantity"]
	if !ok {
		err = schema.ErrMissingQuantity
		return
	}
	amt, err := h.parseAndCheckAmount(qty)
	if err != nil {
		return
	}

	if err = h.stake(from, amt); err != nil {
		return
	}

	nodeNotice := h.genNodeNotice(from, registry, node)
	stakeNotice := &vmmSchema.ResMessage{
		Target: from,
		Data:   "You staked " + qty,
		Tags: []goarSchema.Tag{
			{Name: "Ticker", Value: h.Info().Ticker},
			{Name: "Action", Value: "Stake-Notice"},
			{Name: "Quantity", Value: qty},
		},
	}
	res.Messages = []*vmmSchema.ResMessage{stakeNotice, nodeNotice}
	res.Cache = map[string]string{}
	maps.Copy(res.Cache, h.cacheChangeBalance(from))
	maps.Copy(res.Cache, h.cacheChangeStake(from))
	return
}

func (h *Token) handleUnstake(from string, params map[string]string) (res vmmSchema.Result, err error) {
	registry, ok := params["Registry"]
	if !ok {
		err = schema.ErrMissingQuantity
		return
	}
	node, err := registryUtils.Decode(params)
	if err != nil {
		return
	}
	if from != node.AccId {
		err = schema.ErrUnauthorized
		return
	}

	qty, ok := params["Quantity"]
	if !ok {
		err = schema.ErrMissingQuantity
		return
	}
	amt, err := h.parseAndCheckAmount(qty)
	if err != nil {
		return
	}

	if err = h.unstake(from, amt); err != nil {
		return
	}

	nodeNotice := h.genNodeNotice(from, registry, node)
	unstakeNotice := &vmmSchema.ResMessage{
		Target: from,
		Data:   "You unstaked " + qty,
		Tags: []goarSchema.Tag{
			{Name: "Ticker", Value: h.Info().Ticker},
			{Name: "Action", Value: "Unstake-Notice"},
			{Name: "Quantity", Value: qty},
		},
	}
	res.Messages = []*vmmSchema.ResMessage{unstakeNotice, nodeNotice}
	res.Cache = map[string]string{}
	maps.Copy(res.Cache, h.cacheChangeBalance(from))
	maps.Copy(res.Cache, h.cacheChangeStake(from))
	return
}

func (h *Token) handleSlash() (res vmmSchema.Result, err error) {
	// todo
	return
}

func (h *Token) genNodeNotice(from, registry string, node registrySchema.Node) (msg *vmmSchema.ResMessage) {
	msg = &vmmSchema.ResMessage{
		Target: registry,
		Tags: []goarSchema.Tag{
			{Name: "Action", Value: "Unregister"},
			{Name: "Acc-Id", Value: from},
		},
	}

	bal, err := h.db.StakeOf(from)
	if err != nil {
		log.Error("generate node notice failed", "err", err)
		return
	}

	if bal.Cmp(h.Info().MinAmount) < 0 {
		return
	}

	return &vmmSchema.ResMessage{
		Target: registry,
		Tags: []goarSchema.Tag{
			{Name: "Action", Value: "Register"},
			{Name: "Acc-Id", Value: from},
			{Name: "Name", Value: node.Name},
			// todo: need rank and generate role
			{Name: "Role", Value: node.Role},
			{Name: "Desc", Value: node.Desc},
			{Name: "URL", Value: node.URL},
		},
	}
}

func (h *Token) parseAndCheckAmount(qty string) (*big.Int, error) {
	amt, ok := new(big.Int).SetString(qty, 10)
	if !ok {
		return nil, schema.ErrInvalidQuantityFormat
	}
	if amt.Sign() < 0 {
		return nil, schema.ErrNegativeQuantity
	}
	return amt, nil
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
		"token-info": string(res),
	}
}

func (h *Token) cacheChangeBalance(updateAccounts ...string) map[string]string {
	cacheMap := make(map[string]string)
	for _, accId := range updateAccounts {
		bal, err := h.db.BalanceOf(accId)
		if err != nil {
			bal = big.NewInt(0)
		}
		cacheMap["balances:"+accId] = bal.String()
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
