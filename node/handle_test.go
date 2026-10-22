package node

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/db/cache"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/vmm"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RegistryAuthTestSuite struct {
	suite.Suite
	n           *Node
	outsider    *goar.Bundler
	otherNode   *goar.Bundler
	results     chan vmmSchema.VmmResult
	registryPid string
	tokenPid    string
}

type registryAuthDB struct {
	lifecycleDB
	spawn goarSchema.BundleItem
}

func (db *registryAuthDB) GetMessage(id string) (*goarSchema.BundleItem, error) {
	if id == db.spawn.Id {
		return &db.spawn, nil
	}
	return nil, nil
}

func (suite *RegistryAuthTestSuite) newBundler() *goar.Bundler {
	key, err := crypto.GenerateKey()
	require.NoError(suite.T(), err)
	signer, err := goether.NewSigner(hexutil.Encode(crypto.FromECDSA(key)))
	require.NoError(suite.T(), err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(suite.T(), err)
	return bundler
}

func (suite *RegistryAuthTestSuite) SetupTest() {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(suite.T(), err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(suite.T(), err)
	suite.outsider = suite.newBundler()
	suite.otherNode = suite.newBundler()
	suite.registryPid = strings.Repeat("r", 43)
	suite.tokenPid = strings.Repeat("t", 43)
	suite.results = make(chan vmmSchema.VmmResult, 10)
	suite.n = &Node{
		info:          &nodeSchema.Info{Node: registrySchema.Node{AccId: bundler.Address}},
		bundler:       bundler,
		db:            &registryAuthDB{},
		assignMesChan: make(chan nodeSchema.AssignMessage, 1),
	}
	suite.n.vmm = vmm.New(nil, suite.n.info, suite.results, nil, nil)
	suite.n.vmm.Run()
	suite.T().Cleanup(suite.n.vmm.Close)

	db := cache.NewRegistry(suite.registryPid, suite.tokenPid, registrySchema.Node{AccId: bundler.Address})
	require.NoError(suite.T(), db.RegisterProcess(bundler.Address, suite.registryPid))
	require.NoError(suite.T(), db.RegisterProcess(bundler.Address, suite.tokenPid))
	require.NoError(suite.T(), db.Register(registrySchema.Node{AccId: suite.otherNode.Address}))
	require.NoError(suite.T(), db.RegisterProcess(suite.otherNode.Address, "other-process"))
	data, err := db.Checkpoint()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), suite.n.vmm.Restore(vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: suite.registryPid, ItemId: suite.registryPid, AccId: bundler.Address,
				Params: map[string]string{"Token-Pid": suite.tokenPid, "URL": "http://127.0.0.1:8080"},
			},
			Module: hymxSchema.Module{ModuleFormat: vmmSchema.ModuleFormatRegistry},
		},
		Data: data,
	}))
}

func (suite *RegistryAuthTestSuite) message(bundler *goar.Bundler, action, from, sequence string, params ...goarSchema.Tag) goarSchema.BundleItem {
	tags, err := utils.MessageToTags(hymxSchema.Message{
		Base: hymxSchema.DefaultBaseMessage, Action: action, FromProcess: from, Sequence: sequence,
		Tags: params,
	})
	require.NoError(suite.T(), err)
	item, err := bundler.CreateAndSignItem(nil, suite.registryPid, "", tags)
	require.NoError(suite.T(), err)
	return item
}

func (suite *RegistryAuthTestSuite) TestHandleRejectsUnauthorizedRegistryActions() {
	for _, signer := range []struct {
		name    string
		bundler *goar.Bundler
	}{
		{name: "unregistered wallet", bundler: suite.outsider},
		{name: "node hosting another process", bundler: suite.otherNode},
	} {
		for _, action := range []string{"Register", "Unregister", "UnregisterProcess", "Unknown", ""} {
			suite.Run(signer.name+"/"+action, func() {
				defer func() {
					select {
					case <-suite.n.assignMesChan:
					default:
					}
				}()
				item := suite.message(signer.bundler, action, suite.tokenPid, "1000000")

				err := suite.n.Handle(item)

				require.ErrorIs(suite.T(), err, nodeSchema.ErrUnauthorizedNode)
				assert.Empty(suite.T(), suite.n.assignMesChan)
				snap, err := suite.n.vmm.Checkpoint(suite.registryPid)
				require.NoError(suite.T(), err)
				assert.Zero(suite.T(), snap.Env.Nonce)
				assert.Empty(suite.T(), snap.Env.ReceivedSeq)
			})
		}
	}
}

func (suite *RegistryAuthTestSuite) TestHandleAllowsAuthorizedRegistryActions() {
	for _, action := range []string{"Register", "Unregister", "UnregisterProcess", "Unknown"} {
		item := suite.message(suite.n.bundler, action, suite.tokenPid, "1")

		require.NoError(suite.T(), suite.n.Handle(item))

		require.Len(suite.T(), suite.n.assignMesChan, 1)
		assert.Equal(suite.T(), item.Id, (<-suite.n.assignMesChan).Item.Id)
	}
}

func (suite *RegistryAuthTestSuite) TestRejectedHighSequenceDoesNotBlockTokenRegister() {
	item := suite.message(suite.outsider, "Unknown", suite.tokenPid, "1000000")
	require.ErrorIs(suite.T(), suite.n.Handle(item), nodeSchema.ErrUnauthorizedNode)
	require.Empty(suite.T(), suite.n.assignMesChan)
	item = suite.message(suite.n.bundler, "Register", suite.tokenPid, "1",
		goarSchema.Tag{Name: "Acc-Id", Value: suite.outsider.Address},
		goarSchema.Tag{Name: "Name", Value: "registered"},
		goarSchema.Tag{Name: "Desc", Value: "local test"},
		goarSchema.Tag{Name: "URL", Value: "http://127.0.0.1:8080"},
	)

	require.NoError(suite.T(), suite.n.Handle(item))
	require.Len(suite.T(), suite.n.assignMesChan, 1)
	assigned := <-suite.n.assignMesChan
	require.NoError(suite.T(), suite.n.applyMessage(assigned.Pid, assigned.AccId, assigned.Item, assigned.Message,
		hymxSchema.Assignment{Nonce: "1", Timestamp: "1"}, vmmSchema.ExecModeApply, 0))
	snap, err := suite.n.vmm.Checkpoint(suite.registryPid)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), int64(1), snap.Env.Nonce)
	assert.Equal(suite.T(), int64(1), snap.Env.ReceivedSeq[suite.tokenPid])
	require.Len(suite.T(), suite.results, 1)
	assert.Empty(suite.T(), (<-suite.results).Error)
	node, err := suite.n.GetNode(suite.outsider.Address)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), node)
	assert.Equal(suite.T(), "registered", node.Name)
}

func (suite *RegistryAuthTestSuite) TestRegisterProcessKeepsSpawnVerification() {
	tags, err := utils.ProcessToTags(hymxSchema.Process{
		Base: hymxSchema.DefaultBaseProcess, Module: "module", Scheduler: suite.n.bundler.Address,
	})
	require.NoError(suite.T(), err)
	spawn, err := suite.n.bundler.CreateAndSignItem(nil, "", "", tags)
	require.NoError(suite.T(), err)
	suite.n.db.(*registryAuthDB).spawn = spawn
	item := suite.message(suite.n.bundler, "RegisterProcess", spawn.Id, "0",
		goarSchema.Tag{Name: "Pid", Value: spawn.Id},
		goarSchema.Tag{Name: "Acc-Id", Value: suite.n.bundler.Address},
	)

	assert.ErrorIs(suite.T(), suite.n.authNode(suite.n.bundler.Address, spawn.Id), nodeSchema.ErrUnauthorizedNode)
	require.NoError(suite.T(), suite.n.Handle(item))
	require.Len(suite.T(), suite.n.assignMesChan, 1)
	assert.Equal(suite.T(), item.Id, (<-suite.n.assignMesChan).Item.Id)
	invalid := suite.message(suite.n.bundler, "RegisterProcess", spawn.Id, "0",
		goarSchema.Tag{Name: "Pid", Value: spawn.Id},
		goarSchema.Tag{Name: "Acc-Id", Value: suite.otherNode.Address},
	)
	assert.ErrorIs(suite.T(), suite.n.Handle(invalid), nodeSchema.ErrUnauthorizedNode)
	assert.Empty(suite.T(), suite.n.assignMesChan)
}

func TestRegistryAuthTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryAuthTestSuite))
}
