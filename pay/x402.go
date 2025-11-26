package pay

import "github.com/hymatrix/hymx/pay/schema"

func (p *Pay) X402(url string) *schema.X402Response {
	return &schema.X402Response{
		X402Version: "1.0",
		Error:       "Daily free limit exceeded",
		Accepts: []schema.PaymentOption{
			{
				Scheme:   "exact",
				Network:  "hymatrix",
				Resource: url,
				PayTo:    p.Address(),
				Asset:    p.config.AxToken,
				// SpawnFee covers both the transaction fee and the request to spawn a new vm
				Amount: p.config.SpawnFee.String(),
			},
		},
	}
}
