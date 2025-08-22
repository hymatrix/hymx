package pay

import (
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/pay/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar"
)

var log = common.NewLog("node")

type Pay struct {
	sdk *sdk.SDK

	config *schema.Config
	db     schema.IDB
}

func New(url string, bundler *goar.Bundler, config *schema.Config) *Pay {
	return &Pay{
		sdk: sdk.NewFromBundler(url, bundler),

		config: config,
		db:     cache.NewPay(),
	}
}

func (p *Pay) Run() {}

func (p *Pay) Close() error {
	return nil
}

func (p *Pay) Address() string {
	return p.sdk.GetAddress()
}

func (p *Pay) SaveCheckpoint() error {
	return nil
}

func (p *Pay) LoadCheckpoint() error {
	return nil
}
