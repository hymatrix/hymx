package registry

import (
	"testing"

	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRegistryPid = "registry-pid"
	testTokenPid    = "token-pid"
	testMainAccId   = "0xMain"
	testNodeAccId   = "0xNode"
	testOtherAccId  = "0xOther"
	testProcPid     = "proc-pid"
)

func newTestRegistry(t *testing.T) *Registry {
	db := cache.NewRegistry(testRegistryPid, testTokenPid, schema.Node{AccId: testMainAccId})
	require.NoError(t, db.RegisterProcess(testMainAccId, testTokenPid))
	require.NoError(t, db.Register(schema.Node{AccId: testNodeAccId}))
	require.NoError(t, db.RegisterProcess(testNodeAccId, testProcPid))
	r, err := New(db)
	require.NoError(t, err)
	return r
}

func TestAuthSource(t *testing.T) {
	r := newTestRegistry(t)
	procParams := func(pid, accid string) map[string]string {
		return map[string]string{"Pid": pid, "Acc-Id": accid}
	}

	tests := []struct {
		name string
		meta vmmSchema.Meta
		err  error
	}{
		{name: "no from process", meta: vmmSchema.Meta{AccId: testOtherAccId, Action: "Register"}},
		{name: "token notice from token host", meta: vmmSchema.Meta{AccId: testMainAccId, FromProcess: testTokenPid, Action: "Register"}},
		{name: "token notice from other wallet", meta: vmmSchema.Meta{AccId: testOtherAccId, FromProcess: testTokenPid, Action: "Register"}, err: schema.ErrUnauthorized},
		{name: "token notice from node hosting another process", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testTokenPid, Action: "Unregister"}, err: schema.ErrUnauthorized},
		{name: "unknown action from unregistered process", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: "unknown-pid", Action: "Unknown"}, err: schema.ErrUnauthorized},
		{name: "register process by its node", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: "new-pid", Action: "RegisterProcess", Params: procParams("new-pid", testNodeAccId)}},
		{name: "unregister process by its node", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testProcPid, Action: "UnregisterProcess", Params: procParams(testProcPid, testNodeAccId)}},
		{name: "register process with spoofed from", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testTokenPid, Action: "RegisterProcess", Params: procParams("new-pid", testNodeAccId)}, err: schema.ErrUnauthorized},
		{name: "register process for another node", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: "new-pid", Action: "RegisterProcess", Params: procParams("new-pid", testMainAccId)}, err: schema.ErrUnauthorized},
		{name: "register process by unregistered signer", meta: vmmSchema.Meta{AccId: testOtherAccId, FromProcess: "new-pid", Action: "RegisterProcess", Params: procParams("new-pid", testOtherAccId)}, err: schema.ErrNodeNotFound},
		{name: "re-register process by its host", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testProcPid, Action: "RegisterProcess", Params: procParams(testProcPid, testNodeAccId)}},
		{name: "register process hosted by another node", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testTokenPid, Action: "RegisterProcess", Params: procParams(testTokenPid, testNodeAccId)}, err: schema.ErrUnauthorized},
		{name: "unregister process by registered non-host", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: testTokenPid, Action: "UnregisterProcess", Params: procParams(testTokenPid, testNodeAccId)}, err: schema.ErrUnauthorized},
		{name: "unregister unknown process", meta: vmmSchema.Meta{AccId: testNodeAccId, FromProcess: "new-pid", Action: "UnregisterProcess", Params: procParams("new-pid", testNodeAccId)}, err: schema.ErrUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.AuthSource(tt.meta)
			if tt.err == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.err)
		})
	}
}
