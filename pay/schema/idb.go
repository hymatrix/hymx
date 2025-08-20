package schema

import "math/big"

type IDB interface {
	//////////////////////
	// read only
	IsWhitelist(accid string) bool
	IsExecuted(txHash string) bool

	SponsorTotal(sponsor string) *big.Int
	SponsorBreakdown(sponsor string) map[string]*big.Int

	BeneficiaryTotal(beneficiary string) *big.Int
	BeneficiaryBreakdown(beneficiary string) map[string]*big.Int

	//////////////////////
	// write
	SetWhitelist(accid string, enabled bool) error
	MarkExecuted(txHash string) error

	Deposit(sponsor, beneficiary string, amount *big.Int) error
	Withdraw(sponsor, beneficiary string, amount *big.Int) error

	UseOnce(beneficiary, pid string, fee *big.Int) error
	SpawnFee(beneficiary string, fee *big.Int) error
	ResidencyFee(beneficiary string, fee *big.Int) error
	// devFees pid -> fee
	SettleTxFee(beneficiary string, devShareRatio *big.Int) (nodeFee *big.Int, devFees map[string]*big.Int, err error)

	//////////////////////
	Checkpoint() (data string, err error)
	Restore(data string) error
}
