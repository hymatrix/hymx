package pay

import (
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/pay/schema"
	"github.com/hymatrix/hymx/sdk"
)

var log = common.NewLog("node")

type Pay struct {
	sdk *sdk.SDK

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
	return p.sdk.GetAddress()
}

func (p *Pay) Checkpoint() (string, error) {
	return "", nil
}

func (p *Pay) Restore(data string) error {
	return nil
}
