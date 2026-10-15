package node

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hymatrix/hymx/vmm"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/panjf2000/ants/v2"
	goarSchema "github.com/permadao/goar/schema"
)

type recoveryOrderDB struct {
	lifecycleDB
	messages map[string]*goarSchema.BundleItem
	pids     []string
	nonces   []int64
	visited  chan string
}

func (db *recoveryOrderDB) GetAllProcess() ([]string, []int64, error) {
	if db.nonces != nil {
		return db.pids, db.nonces, nil
	}
	return db.pids, make([]int64, len(db.pids)), nil
}

func (db *recoveryOrderDB) GetCheckpointIndex(pid string) (string, error) {
	if db.visited != nil {
		db.visited <- pid
	}
	return "", nil
}

func (db *recoveryOrderDB) GetMessageByNonce(pid string, nonce int64) (*goarSchema.BundleItem, error) {
	return db.messages[pid], nil
}

func TestRecoveryOrdersCoreBeforeWaitingVMs(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("mod", 0755); err != nil {
		t.Fatal(err)
	}
	db := &recoveryOrderDB{messages: map[string]*goarSchema.BundleItem{}}
	for id, format := range map[string]string{
		"registry": vmmSchema.ModuleFormatRegistry,
		"token":    vmmSchema.ModuleFormatToken,
		"ordinary": "test.runtime",
	} {
		item := goarSchema.BundleItem{Tags: []goarSchema.Tag{{Name: "Module-Format", Value: format}}}
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("mod", "mod-"+id+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		for _, pid := range []string{id, id + "-second"} {
			db.messages[pid] = &goarSchema.BundleItem{Tags: []goarSchema.Tag{{Name: "Module", Value: id}}}
		}
	}
	pids := []string{"ordinary", "token", "registry", "ordinary-second"}
	got, err := (&Node{db: db}).recoveryOrder(pids)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2, 1, 0, 3}; !reflect.DeepEqual(got.order, want) {
		t.Fatalf("recovery order %v, want %v", got.order, want)
	}
}

func TestRecoveryRejectsUnclassifiableDependencies(t *testing.T) {
	n := &Node{db: &recoveryOrderDB{}}
	if _, err := n.recoveryOrder([]string{"missing-spawn"}); err == nil {
		t.Fatal("missing dependencies must fail before a worker can block on registry")
	}
}

func TestRecoveryContinuesAfterMissingModule(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestModule(t, "good-module", false, 0)
	db := &recoveryOrderDB{
		pids:    []string{"bad-pid", "good-pid"},
		nonces:  []int64{-1, -1},
		visited: make(chan string, 2),
		messages: map[string]*goarSchema.BundleItem{
			"bad-pid":  {Tags: []goarSchema.Tag{{Name: "Module", Value: "missing-module"}}},
			"good-pid": {Tags: []goarSchema.Tag{{Name: "Module", Value: "good-module"}}},
		},
	}
	pool, err := ants.NewPool(1)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	ready := make(chan struct{})
	close(ready)
	n := &Node{db: db, ctx: context.Background(), recoveryTaskPool: pool, registrySpawned: ready}
	n.vmm = vmm.New(nil, nil, nil, nil, nil)
	if err := n.runRecovery(); err == nil {
		t.Error("missing module must still be reported")
	}
	select {
	case pid := <-db.visited:
		if pid != "good-pid" {
			t.Fatalf("unexpected recovered process %s", pid)
		}
	default:
		t.Fatal("one missing module prevented the valid process from recovering")
	}
}

func TestFailedRegistrySkipsDependentsButRunsIndependentCore(t *testing.T) {
	t.Chdir(t.TempDir())
	for id, format := range map[string]string{
		"registry-module": vmmSchema.ModuleFormatRegistry,
		"token-module":    vmmSchema.ModuleFormatToken,
		"ordinary-module": "test.runtime",
	} {
		writeTestModule(t, id, false, 0)
		path := filepath.Join("mod", "mod-"+id+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "test.runtime", format)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	db := &recoveryOrderDB{
		pids:    []string{"ordinary-pid", "registry-pid", "token-pid"},
		nonces:  []int64{0, 0, -1},
		visited: make(chan string, 3),
		messages: map[string]*goarSchema.BundleItem{
			"ordinary-pid": {Tags: []goarSchema.Tag{{Name: "Module", Value: "ordinary-module"}}},
			"registry-pid": {Tags: []goarSchema.Tag{{Name: "Module", Value: "registry-module"}}},
			"token-pid":    {Tags: []goarSchema.Tag{{Name: "Module", Value: "token-module"}}},
		},
	}
	pool, err := ants.NewPool(2)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	registryReady := make(chan struct{})
	n := &Node{db: db, ctx: context.Background(), recoveryTaskPool: pool, registrySpawned: registryReady}
	n.vmm = vmm.New(nil, nil, nil, nil, registryReady)
	if err := n.runRecovery(); err == nil {
		t.Fatal("failed registry/dependent jobs must be reported")
	}
	visited := map[string]bool{}
	for len(db.visited) > 0 {
		visited[<-db.visited] = true
	}
	if !visited["registry-pid"] || !visited["token-pid"] || visited["ordinary-pid"] {
		t.Fatalf("unexpected recovery jobs after registry failure: %v", visited)
	}
}

type failedTransactionDB struct {
	lifecycleDB
	requested []int64
}

func (db *failedTransactionDB) GetMessageByNonce(_ string, nonce int64) (*goarSchema.BundleItem, error) {
	db.requested = append(db.requested, nonce)
	return &goarSchema.BundleItem{}, nil
}

func (db *failedTransactionDB) GetAssignByNonce(_ string, _ int64) (*goarSchema.BundleItem, error) {
	return &goarSchema.BundleItem{}, nil
}

func TestRecoveryStopsProcessAtFirstFailedTransaction(t *testing.T) {
	t.Chdir(t.TempDir())
	db := &failedTransactionDB{}
	n := &Node{db: db, ctx: context.Background()}
	n.vmm = vmm.New(nil, nil, nil, nil, nil)
	if err := n.recoveryProcess("failed-pid", 1, "", vmmSchema.ExecModeDryRun); err == nil {
		t.Fatal("invalid transaction must fail its process")
	}
	if !reflect.DeepEqual(db.requested, []int64{0}) {
		t.Fatalf("recovery advanced past the failed transaction: %v", db.requested)
	}
	if !n.vmm.IsRecovering("failed-pid") {
		t.Fatal("partially recovered process must stay locked against new traffic")
	}
}

func TestRecoveryMissingAssignmentReturnsError(t *testing.T) {
	db := &recoveryOrderDB{messages: map[string]*goarSchema.BundleItem{"pid": {}}}
	n := &Node{db: db}
	if _, _, err := n.getMessageAndAssignByNonce("pid", 0); err == nil {
		t.Fatal("a missing assignment must not be returned as successful recovery data")
	}
}

func TestRecoverySubmissionFailureDoesNotHang(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestModule(t, "ordinary", false, 0)
	db := &recoveryOrderDB{
		pids: []string{"ordinary-pid"},
		messages: map[string]*goarSchema.BundleItem{
			"ordinary-pid": {Tags: []goarSchema.Tag{{Name: "Module", Value: "ordinary"}}},
		},
	}
	pool, err := ants.NewPool(DefaultRecoveryConcurrency)
	if err != nil {
		t.Fatal(err)
	}
	pool.Release()
	ready := make(chan struct{})
	close(ready)
	n := &Node{db: db, ctx: context.Background(), recoveryTaskPool: pool, registrySpawned: ready}
	n.vmm = vmm.New(nil, nil, nil, nil, nil)
	done := make(chan error, 1)
	go func() { done <- n.runRecovery() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("rejected recovery task must be reported as an error")
		}
	case <-time.After(time.Second):
		t.Fatal("recovery blocked after its worker pool rejected submission")
	}
}

func TestRecoveryConcurrencyOptions(t *testing.T) {
	config := nodeOptions{recoveryConcurrency: DefaultRecoveryConcurrency}
	if config.recoveryConcurrency != 1 {
		t.Fatal("default recovery must be serialized")
	}
	WithRecoveryConcurrency(3)(&config)
	if config.recoveryConcurrency != 3 {
		t.Fatal("recovery override was ignored")
	}
	defer func() {
		if recover() == nil {
			t.Error("invalid recovery limit accepted")
		}
	}()
	WithRecoveryConcurrency(0)
}
