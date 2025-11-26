package pay

import (
	"math/big"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/pay/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

// hymx fee
func (p *Pay) HymxFeeHandler(itemMeta nodeSchema.ItemMeta) error {
	payer := itemMeta.Signer
	if itemMeta.FromProcess != "" {
		payer = itemMeta.FromProcess
	}

	if p.db.IsWhitelist(payer) {
		return nil
	}

	if p.config.DailyLimit > 0 && p.db.DailyUsage(payer) < p.config.DailyLimit {
		if err := p.db.IncrDailyUsage(payer); err != nil {
			log.Warn("unable to process payment for this transaction", "payer", payer, "itemMeta", itemMeta, "err", err)
			return schema.ErrPaymentFailed
		}
		return nil
	}

	var err error
	switch itemMeta.Instance.(type) {
	case hymxSchema.Process:
		err = p.db.SpawnFee(payer, itemMeta.Pid, p.config.SpawnFee)
	case hymxSchema.Message:
		err = p.db.UseOnce(payer, itemMeta.Pid, p.config.TxFee)
	default:
		err = nodeSchema.ErrInvalidType
	}
	if err != nil {
		log.Warn("unable to process payment for this transaction", "payer", payer, "itemMeta", itemMeta, "err", err)
		return schema.ErrPaymentFailed
	}

	return nil
}

// hmxy Deposit
func (p *Pay) HymxDepositHandler(res vmmSchema.VmmResult) {
	if res.FromProcess != p.config.AxToken {
		return
	}

	tagsList := [][]goarSchema.Tag{}
	for _, msg := range res.Messages {
		if msg.Target == p.Address() && utils.GetTagsValue("Action", msg.Tags) == "Credit-Notice" {
			tagsList = append(tagsList, msg.Tags)
		}
	}

	if len(tagsList) == 0 {
		return
	}

	for _, tags := range tagsList {
		sender := utils.GetTagsValue("Sender", tags)
		qtyStr := utils.GetTagsValue("Quantity", tags)

		qty, ok := new(big.Int).SetString(qtyStr, 10)
		if !ok {
			log.Error("invalid quantity", "item", res.ItemId, "sender", sender, "quantity", qtyStr)
			continue
		}

		beneficiary, err := p.getHymxBeneficiary(sender, res.ItemId)
		if err != nil {
			log.Warn("can not get beneficiary", "item", res.ItemId, "sender", sender, "err", err)
			beneficiary = sender
		}
		p.handleDeposit(res.ItemId, p.config.AxToken, sender, beneficiary, qty)

		log.Info("deposit is successfully!", "sender", sender, "beneficiary", beneficiary, "amount", qty)
	}
}

func (p *Pay) getHymxBeneficiary(sender, itemId string) (beneficiary string, err error) {
	item, err := p.sdk.Client.GetMessage(itemId)
	if err != nil {
		return
	}

	if err = goarUtils.VerifyBundleItem(item); err != nil {
		return
	}

	beneficiary = utils.GetTagsValue("Beneficiary", item.Tags)
	if beneficiary == "" {
		beneficiary = sender
	}

	return
}
