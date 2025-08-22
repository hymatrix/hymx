package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/hymatrix/hymx/db/cache/schema"
)

// Pay provides an in-memory implementation of the payment
// settlement logic.
//
// NOTE:
//   - This version is fully in-memory (maps + big.Int), no persistence.
//   - It is intended only for unit tests, logic verification, and prototyping.
//   - It is NOT suitable for production use.
//
// For production environments, replace this with a persistent and transactional
// implementation (e.g. backed by a database).
type Pay struct {
	whitelist        map[string]bool
	executed         map[string]bool                // txHash -> executed?
	ledger           map[string]map[string]*big.Int // sponsor -> beneficiary -> amount
	txPending        map[string]map[string]*big.Int // beneficiary -> pid -> amount
	spawnPending     map[string]map[string]*big.Int // beneficiary -> pid -> amount
	residencyPending map[string]*big.Int            // benficiary(pid) -> amount

	mu sync.RWMutex
}

func NewPay() *Pay {
	return &Pay{
		whitelist:        map[string]bool{},
		executed:         map[string]bool{},
		ledger:           map[string]map[string]*big.Int{},
		txPending:        map[string]map[string]*big.Int{},
		spawnPending:     map[string]map[string]*big.Int{},
		residencyPending: map[string]*big.Int{},
	}
}

func (p *Pay) IsWhitelist(accid string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.whitelist[accid]
}

func (p *Pay) SetWhitelist(accid string, enabled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.whitelist[accid] = enabled
	return nil
}

func (p *Pay) IsExecuted(txHash string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.executed[txHash]
}
func (p *Pay) MarkExecuted(txHash string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.executed[txHash] {
		return fmt.Errorf("tx already executed: %v", txHash)
	}
	p.executed[txHash] = true
	return nil
}

func (p *Pay) SponsorTotal(sponsor string) *big.Int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	sum := new(big.Int)
	if row, ok := p.ledger[sponsor]; ok {
		for _, v := range row {
			sum.Add(sum, v)
		}
	}
	return sum
}

func (p *Pay) SponsorBreakdown(sponsor string) map[string]*big.Int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make(map[string]*big.Int)
	if row, ok := p.ledger[sponsor]; ok {
		for b, v := range row {
			out[b] = new(big.Int).Set(v)
		}
	}
	return out
}

func (p *Pay) BeneficiaryTotal(beneficiary string) *big.Int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	sum := new(big.Int)
	for _, row := range p.ledger {
		if v, ok := row[beneficiary]; ok {
			sum.Add(sum, v)
		}
	}
	return sum
}

func (p *Pay) BeneficiaryBreakdown(beneficiary string) map[string]*big.Int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make(map[string]*big.Int)
	for s, row := range p.ledger {
		if v, ok := row[beneficiary]; ok && v.Sign() > 0 {
			out[s] = new(big.Int).Set(v)
		}
	}
	return out
}

func (p *Pay) Deposit(sponsor, beneficiary string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount: %v", amount)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	row := p.ledger[sponsor]
	if row == nil {
		row = make(map[string]*big.Int)
		p.ledger[sponsor] = row
	}
	cur := row[beneficiary]
	if cur == nil {
		cur = new(big.Int)
		row[beneficiary] = cur
	}
	cur.Add(cur, amount)
	return nil
}

func (p *Pay) Withdraw(sponsor, beneficiary string, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount: %v", amount)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	row := p.ledger[sponsor]
	if row == nil {
		return fmt.Errorf("insufficient, sponsor: %v, beneficiary: %v", sponsor, beneficiary)
	}
	cur := row[beneficiary]
	if cur == nil || cur.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient, sponsor: %v, beneficiary: %v", sponsor, beneficiary)
	}

	// check pending constraint: beneficiary’s available = total - pending
	totalBal := p.totalBal(beneficiary)
	totalPending := p.totalPending(beneficiary)
	available := new(big.Int).Sub(totalBal, totalPending)
	if available.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient (pending locked), sponsor: %v, beneficiary: %v, amount: %v, available: %v", sponsor, beneficiary, amount, available)
	}

	cur.Sub(cur, amount)
	if cur.Sign() == 0 {
		delete(row, beneficiary)
	}
	if len(row) == 0 {
		delete(p.ledger, sponsor)
	}
	return nil
}

func (p *Pay) CanCover(beneficiary string, delta *big.Int) bool {
	if beneficiary == "" || delta == nil || delta.Sign() <= 0 {
		return false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	avail := p.totalBal(beneficiary)
	pending := p.totalPending(beneficiary)

	remain := new(big.Int).Sub(avail, pending)
	return remain.Cmp(delta) >= 0
}

func (p *Pay) UseOnce(beneficiary, pid string, fee *big.Int) error {
	if fee == nil || fee.Sign() <= 0 {
		return fmt.Errorf("invalid fee: %v", fee)
	}
	if !p.CanCover(beneficiary, fee) {
		return fmt.Errorf("insufficient, beneficiary: %v, fee: %d", beneficiary, fee)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	row := p.txPending[beneficiary]
	if row == nil {
		row = make(map[string]*big.Int)
		p.txPending[beneficiary] = row
	}
	cur := row[pid]
	if cur == nil {
		cur = new(big.Int)
		row[pid] = cur
	}
	cur.Add(cur, fee)
	return nil
}

func (p *Pay) SpawnFee(beneficiary, pid string, fee *big.Int) error {
	if fee == nil || fee.Sign() <= 0 {
		return fmt.Errorf("invalid fee: %v", fee)
	}
	if !p.CanCover(beneficiary, fee) {
		return fmt.Errorf("insufficient, beneficiary: %v, fee: %d", beneficiary, fee)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	row := p.spawnPending[beneficiary]
	if row == nil {
		row = make(map[string]*big.Int)
		p.spawnPending[beneficiary] = row
	}
	cur := row[pid]
	if cur == nil {
		cur = new(big.Int)
		row[pid] = cur
	}
	cur.Add(cur, fee)
	return nil
}

func (p *Pay) ResidencyFee(pid string, fee *big.Int) error {
	if fee == nil || fee.Sign() <= 0 {
		return fmt.Errorf("invalid fee: %v", fee)
	}
	if !p.CanCover(pid, fee) {
		return fmt.Errorf("insufficient, pid: %v, fee: %d", pid, fee)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	cur := p.residencyPending[pid]
	if cur == nil {
		cur = new(big.Int)
		p.residencyPending[pid] = cur
	}
	cur.Add(cur, fee)
	return nil
}

func (p *Pay) SettleTxFee(beneficiary string, devShareRatio *big.Int) (nodeFee *big.Int, devFees map[string]*big.Int, err error) {
	if devShareRatio == nil || devShareRatio.Sign() < 0 || devShareRatio.Cmp(big.NewInt(10000)) > 0 {
		return nil, nil, fmt.Errorf("invalid ratio: %d", devShareRatio)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	txRow := p.txPending[beneficiary]
	devFees = make(map[string]*big.Int)
	totalTx := new(big.Int)
	for pid, amt := range txRow {
		if amt == nil || amt.Sign() <= 0 {
			continue
		}
		totalTx.Add(totalTx, amt)
		// dev = amt * ratio / 10000
		dev := new(big.Int).Mul(amt, devShareRatio)
		dev.Quo(dev, big.NewInt(10000))
		if devFees[pid] == nil {
			devFees[pid] = new(big.Int)
		}
		devFees[pid].Add(devFees[pid], dev)
	}

	spawnRow := p.spawnPending[beneficiary]
	totalSpawn := new(big.Int)
	for _, amt := range spawnRow {
		if amt != nil && amt.Sign() > 0 {
			totalSpawn.Add(totalSpawn, amt)
		}
	}

	resAmt := p.residencyPending[beneficiary]
	totalResidency := new(big.Int)
	if resAmt != nil && resAmt.Sign() > 0 {
		totalResidency.Set(resAmt)
	}

	if totalTx.Sign() == 0 && totalSpawn.Sign() == 0 && totalResidency.Sign() == 0 {
		// no pending fee
		return nil, nil, nil
	}

	// check bal
	totalCharge := new(big.Int).Add(totalTx, totalSpawn)
	totalCharge.Add(totalCharge, totalResidency)
	totalBal := p.totalBal(beneficiary)
	if totalBal.Cmp(totalCharge) < 0 {
		return nil, nil, fmt.Errorf("insufficient, beneficiary: %v, totalCharge: %d, totalBal: %d", beneficiary, totalCharge, totalBal)
	}

	// nodeFee
	totalDev := new(big.Int)
	for _, d := range devFees {
		totalDev.Add(totalDev, d)
	}
	nodeFee = new(big.Int).Sub(totalTx, totalDev)
	nodeFee.Add(nodeFee, totalSpawn)
	nodeFee.Add(nodeFee, totalResidency)

	// handle fee
	type entry struct{ s string }
	var order []entry
	if row, ok := p.ledger[beneficiary]; ok {
		if bal, ok2 := row[beneficiary]; ok2 && bal.Sign() > 0 {
			order = append(order, entry{s: beneficiary})
		}
	}
	var others []string
	for s, row := range p.ledger {
		if s == beneficiary {
			continue
		}
		if bal, ok := row[beneficiary]; ok && bal.Sign() > 0 {
			others = append(others, s)
		}
	}
	sort.Strings(others)
	for _, s := range others {
		order = append(order, entry{s: s})
	}

	remaining := new(big.Int).Set(totalCharge)
	for _, e := range order {
		if remaining.Sign() == 0 {
			break
		}
		bal := p.ledger[e.s][beneficiary]
		if bal == nil || bal.Sign() <= 0 {
			continue
		}
		toDeduct := new(big.Int)
		if bal.Cmp(remaining) >= 0 {
			toDeduct.Set(remaining)
		} else {
			toDeduct.Set(bal)
		}
		bal.Sub(bal, toDeduct)
		if bal.Sign() == 0 {
			delete(p.ledger[e.s], beneficiary)
		}
		if len(p.ledger[e.s]) == 0 {
			delete(p.ledger, e.s)
		}
		remaining.Sub(remaining, toDeduct)
	}
	if remaining.Sign() != 0 {
		return nil, nil, errors.New("internal: remaining charge not zero after sequential deduction")
	}

	delete(p.txPending, beneficiary)
	delete(p.spawnPending, beneficiary)
	delete(p.residencyPending, beneficiary)

	return nodeFee, devFees, nil
}

func (p *Pay) Checkpoint() (data string, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	sp := schema.PaySnapshot{
		Whitelist:        p.whitelist,
		Executed:         p.executed,
		Ledger:           p.ledger,
		TxPending:        p.txPending,
		SpawnPending:     p.spawnPending,
		ResidencyPending: p.residencyPending,
	}

	by, err := json.Marshal(sp)
	if err != nil {
		return "", err
	}
	return string(by), nil
}

func (p *Pay) Restore(data string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sp := &schema.PaySnapshot{}
	if err := json.Unmarshal([]byte(data), sp); err != nil {
		return err
	}

	p.whitelist = sp.Whitelist
	p.executed = sp.Executed
	p.ledger = sp.Ledger
	p.txPending = sp.TxPending
	p.spawnPending = sp.SpawnPending
	p.residencyPending = sp.ResidencyPending
	return nil
}

func (p *Pay) totalPending(beneficiary string) *big.Int {
	sum := new(big.Int)
	if row := p.txPending[beneficiary]; row != nil {
		for _, v := range row {
			if v != nil {
				sum.Add(sum, v)
			}
		}
	}
	if row := p.spawnPending[beneficiary]; row != nil {
		for _, v := range row {
			if v != nil {
				sum.Add(sum, v)
			}
		}
	}
	if v, ok := p.residencyPending[beneficiary]; ok && v != nil {
		sum.Add(sum, v)
	}
	return sum
}

func (p *Pay) totalBal(beneficiary string) *big.Int {
	sum := new(big.Int)
	for _, row := range p.ledger {
		if v, ok := row[beneficiary]; ok {
			sum.Add(sum, v)
		}
	}
	return sum
}
