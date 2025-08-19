package pay

import (
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/pay/schema"
	"github.com/hymatrix/hymx/sdk"
)

type Pay struct {
	signer *goether.Signer
	sdk    *sdk.SDK

	config *schema.Config
	db     schema.IDB
}

func New() *Pay {
	return &Pay{}
}

func (p *Pay) Run() {}

func (p *Pay) Close() error {
	return nil
}

func (p *Pay) Address() string {
	return p.signer.Address.String()
}

func (p *Pay) Checkpoint() (string, error) {
	return "", nil
}

func (p *Pay) Restore(data string) error {
	return nil
}
