package vmm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hymatrix/hymx/cryptor"
	"github.com/hymatrix/hymx/db/cache"
	"github.com/hymatrix/hymx/vmm/core/registry"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptParams(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	c, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	encryptedValue, err := c.Encrypt("secret")
	require.NoError(t, err)

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Foo":           "plain",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	assert.Equal(t, "plain", meta.Params["Foo"])
	assert.Equal(t, "secret", meta.Params["Bar"])
	assert.Equal(t, encryptedValue, meta.Params["Encrypted-Bar"])
}

func TestDecryptParamsNilMeta(t *testing.T) {
	v := &Vmm{}

	assert.NotPanics(t, func() {
		v.decryptParams(nil)
	})
}

func TestDecryptParamsSkipsExistingPlaintextParam(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	c, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	encryptedValue, err := c.Encrypt("secret")
	require.NoError(t, err)

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Bar":           "plain-secret",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	assert.Equal(t, "plain-secret", meta.Params["Bar"])
	assert.Equal(t, encryptedValue, meta.Params["Encrypted-Bar"])
}

func TestDecryptParamsUnsupportedNodeKeepsParams(t *testing.T) {
	v := &Vmm{}
	meta := schema.Meta{
		Params: map[string]string{
			"Encrypted-Bar": "secret",
		},
	}

	v.decryptParams(&meta)
	assert.NotContains(t, meta.Params, "Bar")
	assert.Equal(t, "secret", meta.Params["Encrypted-Bar"])
}

func TestApplyCheckRejectsUnauthorizedSourceBeforeSequence(t *testing.T) {
	db := cache.NewRegistry("registry-pid", "token-pid", registrySchema.Node{AccId: "0xMain"})
	require.NoError(t, db.RegisterProcess("0xMain", "token-pid"))
	vm, err := registry.New(db)
	require.NoError(t, err)
	v := &Vmm{}
	env := &schema.Env{ReceivedSeq: map[string]int64{"token-pid": 5}}

	_, err = v.applyCheck(vm, env, schema.Meta{
		Nonce: 1, AccId: "0xOther", FromProcess: "token-pid", Action: "Register", Sequence: 1000000,
	})
	assert.ErrorIs(t, err, registrySchema.ErrUnauthorized)
	assert.Equal(t, int64(1), env.Nonce)
	assert.Equal(t, int64(5), env.ReceivedSeq["token-pid"])

	from, err := v.applyCheck(vm, env, schema.Meta{
		Nonce: 2, AccId: "0xMain", FromProcess: "token-pid", Action: "Register", Sequence: 6,
	})
	require.NoError(t, err)
	assert.Equal(t, "token-pid", from)
	assert.Equal(t, int64(2), env.Nonce)
	assert.Equal(t, int64(6), env.ReceivedSeq["token-pid"])
}

func TestApplyRejectsUnregisterProcessFromNonHost(t *testing.T) {
	db := cache.NewRegistry("registry-pid", "token-pid", registrySchema.Node{AccId: "0xMain"})
	require.NoError(t, db.RegisterProcess("0xMain", "token-pid"))
	require.NoError(t, db.Register(registrySchema.Node{AccId: "0xNode"}))
	vm, err := registry.New(db)
	require.NoError(t, err)
	results := make(chan schema.VmmResult, 2)
	v := &Vmm{resultChan: results}
	env := &schema.Env{ReceivedSeq: map[string]int64{"token-pid": 5}}

	err = v.apply(vm, env, schema.Meta{
		Pid: "registry-pid", Nonce: 1, AccId: "0xNode", FromProcess: "token-pid", Action: "UnregisterProcess",
		Sequence: 1000000, Params: map[string]string{"Pid": "token-pid", "Acc-Id": "0xNode"}, Mode: schema.ExecModeReplay,
	})
	assert.ErrorIs(t, err, registrySchema.ErrUnauthorized)
	assert.Equal(t, registrySchema.ErrUnauthorized.Error(), (<-results).Error)
	assert.Equal(t, int64(1), env.Nonce)
	assert.Equal(t, int64(5), env.ReceivedSeq["token-pid"])

	require.NoError(t, v.apply(vm, env, schema.Meta{
		Pid: "registry-pid", Nonce: 2, AccId: "0xMain", FromProcess: "token-pid", Action: "Register", Sequence: 6,
		Params: map[string]string{"Acc-Id": "0x6aD04c2815aAF4Ef3fC49EE711238FaE1081829D", "Name": "new", "Desc": "new node", "URL": "http://127.0.0.1:8080"},
		Mode:   schema.ExecModeReplay,
	}))
	assert.Empty(t, (<-results).Error)
	assert.Equal(t, int64(6), env.ReceivedSeq["token-pid"])
	node, err := db.GetNode("0x6aD04c2815aAF4Ef3fC49EE711238FaE1081829D")
	require.NoError(t, err)
	assert.NotNil(t, node)
}
