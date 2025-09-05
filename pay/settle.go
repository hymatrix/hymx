package pay

import (
	"math/big"

	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar/schema"
)

func (p *Pay) settleAll() {
	totalNodeFee := new(big.Int)
	totalDevFees := map[string]*big.Int{}

	all := p.db.AllPending()
	for ben := range all {
		nodeFee, devFees, err := p.db.SettleTxFee(ben, p.config.DeveloperShareRatio)
		if err != nil {
			// If any settlement fails (e.g. insufficient balance), abort immediately
			log.Error("settle failed", "beneficiary", ben, "err", err)
			continue
		}

		if nodeFee != nil {
			totalNodeFee.Add(totalNodeFee, nodeFee)
		}

		// Aggregate devFees by pid
		for pid, fee := range devFees {
			if fee == nil || fee.Sign() == 0 {
				continue
			}
			if totalDevFees[pid] == nil {
				totalDevFees[pid] = new(big.Int)
			}
			totalDevFees[pid].Add(totalDevFees[pid], fee)
		}
	}

	p.settle(totalNodeFee, totalDevFees)
}

func (p *Pay) settle(nodeFee *big.Int, devFees map[string]*big.Int) {
	transfer(p.sdk, p.config.AxToken, p.config.SettlementAddress, nodeFee)

	for pid, fee := range devFees {
		// todo: should transfer to pid owner
		transfer(p.sdk, p.config.AxToken, pid, fee)
	}
}

func transfer(from *sdk.SDK, token, to string, amt *big.Int) error {
	_, err := from.SendMessage(token, "",
		[]schema.Tag{
			{Name: "Action", Value: "Transfer"},
			{Name: "Recipient", Value: to},
			{Name: "Quantity", Value: amt.String()},
		})
	return err
}
