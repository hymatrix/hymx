package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/panjf2000/ants/v2"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeCloseTestSuite struct{ suite.Suite }

type closeTestDB struct {
	lifecycleDB
	resultEntered chan struct{}
	outboxEntered chan struct{}
	release       chan struct{}
	finish        chan struct{}
	count         int
	results       []vmmSchema.VmmResult
	outboxes      []goarSchema.BundleItem
}

func (db *closeTestDB) SaveResult(result vmmSchema.VmmResult) error {
	if len(db.results) == 0 {
		close(db.resultEntered)
		<-db.release
	}
	if len(db.results) == db.count-1 {
		<-db.finish
	}
	db.results = append(db.results, result)
	return nil
}

func (db *closeTestDB) PushOutbox(pid, target string, item goarSchema.BundleItem) error {
	if len(db.outboxes) == 0 {
		close(db.outboxEntered)
		<-db.release
	}
	if len(db.outboxes) == db.count-1 {
		<-db.finish
	}
	db.outboxes = append(db.outboxes, item)
	return nil
}

func (db *closeTestDB) PeekOutbox(pid, target string) (*goarSchema.BundleItem, error) {
	return nil, nil // No remote delivery in this persistence test.
}

type closeTestVM struct {
	lifecycleVM
	done chan struct{}
}

func (vm *closeTestVM) Apply(_ string, meta vmmSchema.Meta) vmmSchema.Result {
	return vmmSchema.Result{Messages: []*vmmSchema.ResMessage{{
		Target: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Tags:   []goarSchema.Tag{{Name: "Action", Value: "test"}},
	}}}
}
func (vm *closeTestVM) Close() error {
	close(vm.done)
	return nil
}

func (suite *NodeCloseTestSuite) wait(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(suite.T(), "shutdown timed out")
	}
}

func (suite *NodeCloseTestSuite) TestCloseDrainsOutput() {
	for _, checkpoint := range []bool{false, true} {
		suite.Run(fmt.Sprintf("checkpoint=%t", checkpoint), func() {
			keyfile, err := filepath.Abs("../cmd/test_keyfile.json")
			require.NoError(suite.T(), err)
			signer, err := goar.NewSignerFromPath(keyfile)
			require.NoError(suite.T(), err)
			bundler, err := goar.NewBundler(signer)
			require.NoError(suite.T(), err)
			oldWd, err := os.Getwd()
			require.NoError(suite.T(), err)
			require.NoError(suite.T(), os.Chdir(suite.T().TempDir()))
			suite.T().Cleanup(func() { require.NoError(suite.T(), os.Chdir(oldWd)) })
			pool, err := ants.NewPool(1)
			require.NoError(suite.T(), err)
			db := &closeTestDB{
				resultEntered: make(chan struct{}), outboxEntered: make(chan struct{}),
				release: make(chan struct{}), finish: make(chan struct{}), count: 10,
			}
			ctx, cancel := context.WithCancel(context.Background())
			n := &Node{
				info: &schema.Info{}, bundler: bundler, db: db, ctx: ctx, cancel: cancel,
				resultChan: make(chan vmmSchema.VmmResult, 1), outboxChan: make(chan vmmSchema.Outbox, 1),
				assignResChan:     make(chan schema.AssignmentResult, 1),
				outboxSendingLock: map[string]bool{}, recoveryTaskPool: pool,
			}
			n.vmm = vmm.New(nil, n.info, n.resultChan, n.outboxChan, nil)
			n.outputWg.Add(3)
			go n.runResultChan()
			go n.runOutboxChan()
			go n.runAssignmentChan()
			var releaseOnce, finishOnce, closeOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(db.release) }) }
			finish := func() { finishOnce.Do(func() { close(db.finish) }) }
			shutdown := func() { closeOnce.Do(func() { n.close(checkpoint) }) }
			suite.T().Cleanup(func() { release(); finish(); shutdown() })
			vm := &closeTestVM{done: make(chan struct{})}
			require.NoError(suite.T(), n.vmm.Mount("test", func(vmmSchema.Env) (vmmSchema.Vm, error) { return vm, nil }))
			require.NoError(suite.T(), n.vmm.Restore(vmmSchema.Snapshot{Env: vmmSchema.Env{
				Meta: vmmSchema.Meta{Pid: "a"}, Module: hymxSchema.Module{ModuleFormat: "test"},
			}}))
			// Keep an assignment producer alive until cancellation, like an in-flight input.
			assignments := 0
			n.AddAssignResHandler(func(schema.AssignmentResult) { assignments++ })
			n.wg.Add(1)
			go func() {
				defer n.wg.Done()
				<-ctx.Done()
				for i := 0; i < db.count; i++ {
					n.assignResChan <- schema.AssignmentResult{}
				}
			}()
			for nonce := 1; nonce <= db.count; nonce++ {
				n.vmm.Apply(vmmSchema.Meta{Pid: "a", Nonce: int64(nonce), Mode: vmmSchema.ExecModeApply})
			}
			suite.wait(db.resultEntered)
			suite.wait(db.outboxEntered)
			done := make(chan struct{})
			go func() { shutdown(); close(done) }()
			suite.T().Cleanup(func() { release(); finish(); suite.wait(done) })
			suite.wait(ctx.Done())
			release()
			suite.wait(vm.done)
			// VM completion must not bypass the final result/outbox persistence.
			select {
			case <-done:
				require.FailNow(suite.T(), "Close returned before output was persisted")
			default:
			}
			finish()
			suite.wait(done)
			assert.Len(suite.T(), db.results, db.count)
			assert.Len(suite.T(), db.outboxes, db.count)
			assert.Equal(suite.T(), db.count, assignments)
			for i, result := range db.results {
				assert.Equal(suite.T(), fmt.Sprint(i+1), result.Nonce)
			}
			if checkpoint {
				assert.Equal(suite.T(), 1, vm.checkpoints)
				_, err := LoadCheckpoint(db.saveCheckpointID)
				require.NoError(suite.T(), err)
			} else {
				assert.Zero(suite.T(), vm.checkpoints)
			}
		})
	}
}

func TestNodeCloseTestSuite(t *testing.T) { suite.Run(t, new(NodeCloseTestSuite)) }
