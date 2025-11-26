package cache

import (
	"math/big"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bi(n int64) *big.Int { return big.NewInt(n) }

func TestWhitelistAndExecuted(t *testing.T) {
	p := NewPay()

	// whitelist
	require.NoError(t, p.SetWhitelist("alice", true))
	assert.True(t, p.IsWhitelist("alice"))
	require.NoError(t, p.SetWhitelist("alice", false))
	assert.False(t, p.IsWhitelist("alice"))

	// executed
	assert.False(t, p.IsExecuted("0xabc"))
	require.NoError(t, p.MarkExecuted("0xabc"))
	assert.True(t, p.IsExecuted("0xabc"))
	err := p.MarkExecuted("0xabc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tx already executed")
}

func TestDepositWithdrawAndBreakdowns(t *testing.T) {
	p := NewPay()

	require.NoError(t, p.Deposit("s1", "b1", bi(100)))
	require.NoError(t, p.Deposit("s1", "b2", bi(50)))
	require.NoError(t, p.Deposit("s2", "b1", bi(30)))

	assert.Equal(t, int64(150), p.SponsorTotal("s1").Int64())
	assert.Equal(t, int64(130), p.BeneficiaryTotal("b1").Int64())

	s1bd := p.SponsorBreakdown("s1")
	assert.Equal(t, int64(100), s1bd["b1"].Int64())
	assert.Equal(t, int64(50), s1bd["b2"].Int64())

	b1bd := p.BeneficiaryBreakdown("b1")
	// order not guaranteed; check set
	keys := make([]string, 0, len(b1bd))
	for k := range b1bd {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"s1", "s2"}, keys)

	// withdraw
	require.NoError(t, p.Withdraw("s1", "b1", bi(40)))
	assert.Equal(t, int64(60), p.SponsorBreakdown("s1")["b1"].Int64())

	// insufficient
	err := p.Withdraw("s1", "b1", bi(1000))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestCanCoverAndPendingAccrual(t *testing.T) {
	p := NewPay()
	// sponsor deposits to beneficiary
	require.NoError(t, p.Deposit("alice", "bob", bi(100)))

	// UseOnce ok
	assert.True(t, p.CanCover("bob", bi(10)))
	require.NoError(t, p.UseOnce("bob", "pidA", bi(10)))
	// SpawnFee ok
	require.NoError(t, p.SpawnFee("bob", "pidA", bi(5)))
	// ResidencyFee (pid as beneficiary)
	require.NoError(t, p.Deposit("sX", "pidA", bi(20)))
	require.NoError(t, p.ResidencyFee("pidA", bi(12)))

	// pending totals
	txP := p.txPending["bob"]["pidA"]
	spP := p.spawnPending["bob"]["pidA"]
	rsP := p.residencyPending["pidA"]
	require.NotNil(t, txP)
	require.NotNil(t, spP)
	require.NotNil(t, rsP)
	assert.Equal(t, int64(10), txP.Int64())
	assert.Equal(t, int64(5), spP.Int64())
	assert.Equal(t, int64(12), rsP.Int64())

	// CanCover fails if exceeding (bob has 100; bob pending now 15)
	assert.False(t, p.CanCover("bob", bi(1000)))
}

func TestSettleTxFee_SequentialDeduct_SelfFirst_DevBps(t *testing.T) {
	p := NewPay()

	// Ledger:
	// - Self sponsor: bob->bob = 60
	// - Other sponsors: alice->bob = 50, carl->bob = 70  (alphabetical order: alice, carl)
	require.NoError(t, p.Deposit("bob", "bob", bi(60)))
	require.NoError(t, p.Deposit("carl", "bob", bi(70)))
	require.NoError(t, p.Deposit("alice", "bob", bi(50)))

	// Pending:
	//   txPending[bob]:  pidX=40, pidY=10  => totalTx=50
	//   spawnPending[bob]: pidX=5, pidY=7  => totalSpawn=12
	//   residencyPending[bob]: 8          => totalResidency=8
	// devShareRatio = 500 (== 5%)
	require.NoError(t, p.UseOnce("bob", "pidX", bi(40)))
	require.NoError(t, p.UseOnce("bob", "pidY", bi(10)))
	require.NoError(t, p.SpawnFee("bob", "pidX", bi(5)))
	require.NoError(t, p.SpawnFee("bob", "pidY", bi(7)))
	require.NoError(t, p.ResidencyFee("bob", bi(8)))

	devRatio := bi(500) // 5% in bps
	nodeFee, devFees, err := p.SettleTxFee("bob", devRatio)
	require.NoError(t, err)

	// dev per pid: 5% of each pid's tx amount
	// pidX: 40 * 5% = 2 ; pidY: 10 * 5% = 0 (floor)
	require.Equal(t, int64(2), devFees["pidX"].Int64())
	require.Equal(t, int64(0), devFees["pidY"].Int64())

	// nodeFee = (totalTx - sum(dev)) + totalSpawn + totalResidency
	//         = (50 - 2) + 12 + 8 = 68
	require.Equal(t, int64(68), nodeFee.Int64())

	// Total charge = 50 + 12 + 8 = 70
	// Deduct in order: self bob->bob (60) first, then others by alphabetical (alice->bob 50, carl->bob 70)
	// After deduction of 70:
	//   bob->bob: 60 - 60 = 0
	//   remaining 10 -> next sponsor: alice->bob: 50 - 10 = 40
	// carl untouched.
	assert.Zero(t, p.SponsorBreakdown("bob")["bob"])
	assert.Equal(t, int64(40), p.SponsorBreakdown("alice")["bob"].Int64())
	assert.Equal(t, int64(70), p.SponsorBreakdown("carl")["bob"].Int64())

	// pendings are cleared
	_, ok := p.txPending["bob"]
	assert.False(t, ok)
	_, ok = p.spawnPending["bob"]
	assert.False(t, ok)
	_, ok = p.residencyPending["bob"]
	assert.False(t, ok)
}

func TestSettleTxFee(t *testing.T) {
	p := NewPay()

	// Only 20 available for bob
	require.NoError(t, p.Deposit("s1", "bob", bi(20)))

	// But charge requires 25 (tx=25), then 20 (tx=15, spawn=5)
	err := p.UseOnce("bob", "pid1", bi(25))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
	require.NoError(t, p.UseOnce("bob", "pid1", bi(15)))
	require.NoError(t, p.SpawnFee("bob", "pid1", bi(5)))

	nodeFee, devFees, err := p.SettleTxFee("bob", bi(3000)) // 30%
	require.NoError(t, err)
	assert.Equal(t, int64(16), nodeFee.Int64())
	assert.Equal(t, int64(4), devFees["pid1"].Int64())
}

func TestSettleTxFee_NoPending_ReturnsNil(t *testing.T) {
	p := NewPay()
	// No pending for charlie; should return (nil, nil, nil)
	node, dev, err := p.SettleTxFee("charlie", bi(500))
	require.NoError(t, err)
	assert.Nil(t, node)
	assert.Nil(t, dev)
}

func TestUseOnce_Spawn_Residency_CoverChecks(t *testing.T) {
	p := NewPay()

	// beneficiary dana has 100 total (two sponsors)
	require.NoError(t, p.Deposit("sA", "dana", bi(60)))
	require.NoError(t, p.Deposit("sB", "dana", bi(40)))

	// Can cover exactly 100
	assert.True(t, p.CanCover("dana", bi(100)))
	// But after recording 90 pending, only 10 left
	require.NoError(t, p.UseOnce("dana", "pid1", bi(50)))
	require.NoError(t, p.SpawnFee("dana", "pid1", bi(40)))

	assert.True(t, p.CanCover("dana", bi(10)))
	assert.False(t, p.CanCover("dana", bi(11)))

	// Residency uses pid as beneficiary; need separate deposit to pid
	require.NoError(t, p.Deposit("sX", "pidZ", bi(5)))
	require.NoError(t, p.ResidencyFee("pidZ", bi(5)))
	// Additional residency exceeding balance should fail
	err := p.ResidencyFee("pidZ", bi(1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestDailyUsage(t *testing.T) {
	p := NewPay()

	// 1. Initial usage should be 0 for a new user
	assert.Equal(t, int64(0), p.DailyUsage("user1"))

	// 2. Increment usage count
	require.NoError(t, p.IncrDailyUsage("user1"))
	assert.Equal(t, int64(1), p.DailyUsage("user1"))

	// 3. Increment usage count again
	require.NoError(t, p.IncrDailyUsage("user1"))
	assert.Equal(t, int64(2), p.DailyUsage("user1"))

	// 4. Verify that another user is unaffected
	assert.Equal(t, int64(0), p.DailyUsage("user2"))
	require.NoError(t, p.IncrDailyUsage("user2"))
	assert.Equal(t, int64(1), p.DailyUsage("user2"))
	// user1 should still be 2
	assert.Equal(t, int64(2), p.DailyUsage("user1"))

	// 5. Reset all daily usage counts
	require.NoError(t, p.ResetDailyUsage())

	// 6. Verify that all users' usage counts are reset to 0
	assert.Equal(t, int64(0), p.DailyUsage("user1"))
	assert.Equal(t, int64(0), p.DailyUsage("user2"))
}

func TestWithdraw_CleansUpEmptyMaps(t *testing.T) {
	p := NewPay()
	require.NoError(t, p.Deposit("s", "b", bi(10)))
	require.NoError(t, p.Withdraw("s", "b", bi(10)))

	// sponsor map should be removed
	_, ok := p.ledger["s"]
	assert.False(t, ok)
}

func TestAllPending(t *testing.T) {
	p := NewPay()

	// Prepare ledger: sponsorA -> ben1:100, sponsorB -> ben2:200
	require.NoError(t, p.Deposit("sponsorA", "ben1", bi(100)))
	require.NoError(t, p.Deposit("sponsorB", "ben2", bi(200)))

	// Add pending balances:
	// ben1: txPending=30, spawnPending=20
	require.NoError(t, p.UseOnce("ben1", "pidX", bi(30)))
	require.NoError(t, p.SpawnFee("ben1", "pidX", bi(20)))

	// ben2: residencyPending=50 (here the pid is the same as beneficiary = "ben2")
	require.NoError(t, p.Deposit("sponsorC", "ben2", bi(100)))
	require.NoError(t, p.ResidencyFee("ben2", bi(50)))

	// ben3: txPending=10
	require.NoError(t, p.Deposit("sponsorD", "ben3", bi(100)))
	require.NoError(t, p.UseOnce("ben3", "pidY", bi(10)))

	// Call AllPending
	pending := p.AllPending()

	// Validate ben1 -> 30+20=50
	require.Contains(t, pending, "ben1")
	assert.Equal(t, int64(50), pending["ben1"].Int64())

	// Validate ben2 -> 50
	require.Contains(t, pending, "ben2")
	assert.Equal(t, int64(50), pending["ben2"].Int64())

	// Validate ben3 -> 10
	require.Contains(t, pending, "ben3")
	assert.Equal(t, int64(10), pending["ben3"].Int64())

	// A beneficiary without any pending should not appear
	_, ok := pending["unknown"]
	assert.False(t, ok)
}
