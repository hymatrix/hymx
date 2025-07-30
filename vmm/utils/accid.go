package utils

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar/utils"
)

func IDCheck(id string) (accountType, accId string, err error) {
	if common.IsHexAddress(id) {
		return schema.AccountTypeEVM, common.HexToAddress(id).String(), nil
	}

	if isArAddress(id) {
		return schema.AccountTypeAR, id, nil
	}

	return "", "", schema.ErrInvalidAccid
}

func isArAddress(addr string) bool {
	if len(addr) != 43 {
		return false
	}
	_, err := utils.Base64Decode(addr)
	if err != nil {
		return false
	}

	return true
}
