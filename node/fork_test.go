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
)

func newForkTestBundler(t *testing.T) *goar.Bundler {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer, err := goether.NewSigner(hexutil.Encode(crypto.FromECDSA(key)))
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	return bundler
}

func newForkTestAssign(t *testing.T, bundler *goar.Bundler, pid, msgId, nonce string) goarSchema.BundleItem {
	tags, err := utils.AssignmentToTags(hymxSchema.Assignment{
		Base: hymxSchema.DefaultBaseAssignment, Process: pid, Message: msgId, Nonce: nonce, Timestamp: "1",
	})
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem(nil, "", "", tags)
	require.NoError(t, err)
	return item
}

func TestForkScheduler(t *testing.T) {
	scheduler, user := newForkTestBundler(t), newForkTestBundler(t)
	tags, err := utils.ProcessToTags(hymxSchema.Process{
		Base: hymxSchema.DefaultBaseProcess, Module: "module", Scheduler: scheduler.Address,
	})
	require.NoError(t, err)
	spawn, err := user.CreateAndSignItem(nil, "", "", tags)
	require.NoError(t, err)

	got, err := forkScheduler(spawn.Id, spawn)
	require.NoError(t, err)
	assert.Equal(t, scheduler.Address, got)

	_, err = forkScheduler("another-pid", spawn)
	assert.ErrorIs(t, err, nodeSchema.ErrInvalidForkItem)

	tampered := spawn
	tampered.Data = "eA"
	_, err = forkScheduler(spawn.Id, tampered)
	assert.Error(t, err)
}

func TestVerifyForkItem(t *testing.T) {
	scheduler, user, other := newForkTestBundler(t), newForkTestBundler(t), newForkTestBundler(t)
	pid := strings.Repeat("p", 43)
	tags, err := utils.MessageToTags(hymxSchema.Message{Base: hymxSchema.DefaultBaseMessage, Action: "Info"})
	require.NoError(t, err)
	msg, err := user.CreateAndSignItem(nil, pid, "", tags)
	require.NoError(t, err)

	assign, err := verifyForkItem(pid, 7, scheduler.Address, msg, newForkTestAssign(t, scheduler, pid, msg.Id, "7"))
	require.NoError(t, err)
	assert.Equal(t, "7", assign.Nonce)
	assert.Equal(t, msg.Id, assign.Message)

	tamperedMsg := msg
	tamperedMsg.Data = "eA"

	for _, tt := range []struct {
		name   string
		msg    goarSchema.BundleItem
		assign goarSchema.BundleItem
	}{
		{name: "assignment signed by another node", msg: msg, assign: newForkTestAssign(t, other, pid, msg.Id, "7")},
		{name: "assignment for another process", msg: msg, assign: newForkTestAssign(t, scheduler, strings.Repeat("o", 43), msg.Id, "7")},
		{name: "assignment for another message", msg: msg, assign: newForkTestAssign(t, scheduler, pid, "other-msg", "7")},
		{name: "assignment for another nonce", msg: msg, assign: newForkTestAssign(t, scheduler, pid, msg.Id, "8")},
		{name: "tampered message", msg: tamperedMsg, assign: newForkTestAssign(t, scheduler, pid, msg.Id, "7")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifyForkItem(pid, 7, scheduler.Address, tt.msg, tt.assign)
			assert.Error(t, err)
		})
	}
}
