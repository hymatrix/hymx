package node

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/everFinance/goether"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ForkTestSuite struct{ suite.Suite }

func (suite *ForkTestSuite) bundler() *goar.Bundler {
	key, err := crypto.GenerateKey()
	require.NoError(suite.T(), err)
	signer, err := goether.NewSigner(hexutil.Encode(crypto.FromECDSA(key)))
	require.NoError(suite.T(), err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(suite.T(), err)
	return bundler
}

func (suite *ForkTestSuite) TestForkBindings() {
	scheduler, other := suite.bundler(), suite.bundler()
	tags, err := utils.ProcessToTags(hymxSchema.Process{Base: hymxSchema.DefaultBaseProcess, Module: "module", Scheduler: scheduler.Address})
	require.NoError(suite.T(), err)
	spawn, err := other.CreateAndSignItem(nil, "", "", tags)
	require.NoError(suite.T(), err)
	got, err := forkScheduler(spawn.Id, spawn)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), scheduler.Address, got)
	_, err = forkScheduler("wrong", spawn)
	assert.ErrorIs(suite.T(), err, nodeSchema.ErrInvalidForkItem)
	message := func(target string) goarSchema.BundleItem {
		tags, err := utils.MessageToTags(hymxSchema.Message{Base: hymxSchema.DefaultBaseMessage, Action: "Info"})
		require.NoError(suite.T(), err)
		item, err := other.CreateAndSignItem(nil, target, "", tags)
		require.NoError(suite.T(), err)
		return item
	}
	msg := message(spawn.Id)
	assignment := func(signer *goar.Bundler, pid, id, nonce string) goarSchema.BundleItem {
		tags, err := utils.AssignmentToTags(hymxSchema.Assignment{Base: hymxSchema.DefaultBaseAssignment, Process: pid, Message: id, Nonce: nonce, Timestamp: "1"})
		require.NoError(suite.T(), err)
		item, err := signer.CreateAndSignItem(nil, "", "", tags)
		require.NoError(suite.T(), err)
		return item
	}
	wrongTarget := message(strings.Repeat("a", 43))
	for _, tc := range []struct {
		name        string
		nonce       int64
		msg, assign goarSchema.BundleItem
		valid       bool
	}{
		{name: "spawn", msg: spawn, assign: assignment(scheduler, spawn.Id, spawn.Id, "0"), valid: true},
		{name: "message", nonce: 1, msg: msg, assign: assignment(scheduler, spawn.Id, msg.Id, "1"), valid: true},
		{name: "wrong signer", nonce: 1, msg: msg, assign: assignment(other, spawn.Id, msg.Id, "1")},
		{name: "wrong process", nonce: 1, msg: msg, assign: assignment(scheduler, "other", msg.Id, "1")},
		{name: "wrong message", nonce: 1, msg: msg, assign: assignment(scheduler, spawn.Id, "other", "1")},
		{name: "wrong nonce", nonce: 1, msg: msg, assign: assignment(scheduler, spawn.Id, msg.Id, "2")},
		{name: "wrong target", nonce: 1, msg: wrongTarget, assign: assignment(scheduler, spawn.Id, wrongTarget.Id, "1")},
		{name: "spawn after zero", nonce: 1, msg: spawn, assign: assignment(scheduler, spawn.Id, spawn.Id, "1")},
		{name: "message at zero", msg: msg, assign: assignment(scheduler, spawn.Id, msg.Id, "0")},
		{name: "wrong type", nonce: 1, msg: msg, assign: msg},
	} {
		suite.Run(tc.name, func() {
			_, err := verifyForkItem(spawn.Id, tc.nonce, scheduler.Address, tc.msg, tc.assign)
			if tc.valid {
				assert.NoError(suite.T(), err)
			} else {
				assert.Error(suite.T(), err)
			}
		})
	}
}

func TestForkTestSuite(t *testing.T) { suite.Run(t, new(ForkTestSuite)) }
