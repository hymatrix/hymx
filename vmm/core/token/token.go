package token

import (
	"math/big"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/vmm/core/token/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/hymatrix/hymx/vmm/utils"
)

var log = common.NewLog("token")

type Token struct {
	db schema.IDB
}

func New(db schema.IDB) (*Token, error) {
	return &Token{db}, nil
}

func (h *Token) Apply(from string, meta vmmSchema.Meta) (res vmmSchema.Result) {
	var err error
	switch meta.Action {
	case "Info":
		res, err = h.handleInfo(from)
	case "Total-Supply":
		res, err = h.handleTotalSupply(from)
	case "Balance":
		res, err = h.handleBalanceOf(from, meta.Params)
	case "Balances":
		res, err = h.handleBalances(from)
	case "Transfer":
		res, err = h.handleTransfer(from, meta.Params)
	case "Stake":
		res, err = h.handleStake(from, meta.Params)
	case "Unstake":
		res, err = h.handleUnstake(from, meta.Params)
	case "Slash":
		res, err = h.handleSlash()
	default:
		err = schema.ErrInvalidAction
	}
	if err != nil {
		res.Error = err.Error()
	}

	return
}

func (h *Token) Checkpoint() (string, error) {
	return h.db.Checkpoint()
}

func (h *Token) Restore(data string) error {
	return h.db.Restore(data)
}

func (h *Token) Close() error {
	return nil
}

func (h *Token) Info() schema.Info {
	return h.db.GetInfo()
}

func (h *Token) BalanceOf(accid string) (*big.Int, error) {
	_, accid, err := utils.IDCheck(accid)
	if err != nil {
		return nil, err
	}
	return h.db.BalanceOf(accid)
}

func (h *Token) StakeOf(accid string) (*big.Int, error) {
	_, accid, err := utils.IDCheck(accid)
	if err != nil {
		return nil, err
	}
	return h.db.StakeOf(accid)
}
