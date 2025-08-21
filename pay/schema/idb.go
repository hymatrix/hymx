package schema

import "math/big"

type IDB interface {
	IsWhitelist(accid string) bool
	SetWhitelist(accid string, enabled bool) error

	IsExecuted(txHash string) bool
	MarkExecuted(txHash string) error

	SponsorTotal(sponsor string) *big.Int
	SponsorBreakdown(sponsor string) map[string]*big.Int
	BeneficiaryTotal(beneficiary string) *big.Int
	BeneficiaryBreakdown(beneficiary string) map[string]*big.Int

	Deposit(sponsor, beneficiary string, amount *big.Int) error
	Withdraw(sponsor, beneficiary string, amount *big.Int) error

	CanCover(beneficiary string, delta *big.Int) bool
	UseOnce(beneficiary, pid string, fee *big.Int) error
	SpawnFee(beneficiary, pid string, fee *big.Int) error
	ResidencyFee(pid string, fee *big.Int) error
	// devFees pid -> fee
	SettleTxFee(beneficiary string, devShareRatio *big.Int) (nodeFee *big.Int, devFees map[string]*big.Int, err error)

	//////////////////////
	Checkpoint() (data string, err error)
	Restore(data string) error
}
