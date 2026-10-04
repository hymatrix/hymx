package vmm

import (
	"errors"
	"testing"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// VmmKillTestSuite tests VMM kill behavior.
type VmmKillTestSuite struct {
	suite.Suite
}

type killTestVM struct {
	closed bool
	err    error
}

func (v *killTestVM) Apply(from string, meta schema.Meta) schema.Result { return schema.Result{} }
func (v *killTestVM) Checkpoint() (string, error)                       { return "{}", nil }
func (v *killTestVM) Restore(data string) error                         { return nil }
func (v *killTestVM) Close() error {
	v.closed = true
	return v.err
}

func newKillTestVMM() *Vmm {
	return New(nil, &nodeSchema.Info{}, nil, nil, nil)
}

func (suite *VmmKillTestSuite) TestKillClosesAndRemovesVM() {
	v := newKillTestVMM()
	vm := &killTestVM{}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), vm.closed)
	assert.False(suite.T(), v.IsExists("pid-1"))
	assert.Empty(suite.T(), v.GetVmPids())
}

func (suite *VmmKillTestSuite) TestKillCloseFailureKeepsVM() {
	v := newKillTestVMM()
	vm := &killTestVM{err: errors.New("close failed")}
	v.addVm(vm, &schema.Env{Meta: schema.Meta{Pid: "pid-1"}})

	err := v.Kill("pid-1")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), vm.closed)
	assert.True(suite.T(), v.IsExists("pid-1"))
	assert.Equal(suite.T(), []string{"pid-1"}, v.GetVmPids())
}

func (suite *VmmKillTestSuite) TestKillMissingProcessReturnsProcessNotFound() {
	v := newKillTestVMM()

	err := v.Kill("missing")

	assert.ErrorIs(suite.T(), err, schema.ErrProcessNotFound)
}

func TestVmmKillTestSuite(t *testing.T) {
	suite.Run(t, new(VmmKillTestSuite))
}
