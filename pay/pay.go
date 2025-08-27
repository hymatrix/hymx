package pay

import (
	"math/big"
	"os"
	"path/filepath"

	"github.com/go-co-op/gocron/v2"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/pay/schema"
	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar"
)

var log = common.NewLog("pay")

type Pay struct {
	scheduler gocron.Scheduler

	sdk *sdk.SDK

	config *schema.Config
	db     schema.IDB
}

func New(url string, bundler *goar.Bundler, config *schema.Config) *Pay {
	config.ChargeAddress = bundler.Address
	return &Pay{
		sdk: sdk.NewFromBundler(url, bundler),

		config: config,
		db:     cache.NewPay(),
	}
}

func (p *Pay) Run() {
	p.runJobs()

	log.Info("payment is running", "wallet", p.Address(), "settle", p.config.SettlementAddress)
}

func (p *Pay) Close() error {
	p.scheduler.Shutdown()
	return nil
}

func (p *Pay) Info() schema.Config {
	return *p.config
}

func (p *Pay) Address() string {
	return p.sdk.GetAddress()
}

func (p *Pay) SponsorTotal(sponsor string) *big.Int {
	return p.db.SponsorTotal(sponsor)
}

func (p *Pay) SponsorBreakdown(sponsor string) map[string]*big.Int {
	return p.db.SponsorBreakdown(sponsor)
}

func (p *Pay) BeneficiaryTotal(beneficiary string) *big.Int {
	return p.db.BeneficiaryTotal(beneficiary)
}

func (p *Pay) BeneficiaryBreakdown(beneficiary string) map[string]*big.Int {
	return p.db.BeneficiaryBreakdown(beneficiary)
}

func (p *Pay) TotalPending(beneficiary string) *big.Int {
	return p.db.TotalPending(beneficiary)
}

func (p *Pay) SaveCheckpoint() error {
	if err := os.MkdirAll("./ckp", 0755); err != nil {
		return err
	}

	data, err := p.db.Checkpoint()
	if err != nil {
		return err
	}

	filename := filepath.Join("ckp", "ckp-pay.dump")
	return os.WriteFile(filename, []byte(data), 0644)
}

func (p *Pay) LoadCheckpoint() error {
	filename := filepath.Join("ckp", "ckp-pay.dump")
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return p.db.Restore(string(data))
}
