# Encrypt Tags VMM Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move encrypted tag decryption from node into VMM while preserving raw encrypted tags and exposing decrypted values through `Meta.DecryptedParams`.

**Architecture:** Node verifies, routes, assigns, persists, and checkpoints raw bundle item data without decrypting or renaming encrypted tags. VMM owns encrypted tag decryption through a decryption-key dependency and derives `DecryptedParams` at spawn, apply, and restore boundaries. Checkpoints serialize raw env, VM state, and outbox only.

**Tech Stack:** Go 1.24+, `github.com/permadao/goar`, `github.com/everFinance/goether`, existing `utils/tagcrypto`, `node`, and `vmm` packages.

---

## File Structure

- Modify `utils/tagcrypto/tagcrypto.go`: change prefix to `Encrypted-`; add parameter-map decryption helpers that keep encrypted keys.
- Modify `utils/tagcrypto/tagcrypto_test.go`: replace renamed-tag expectations with decrypted-params expectations.
- Modify `vmm/schema/schema.go`: add `DecryptedParams map[string]string`, then remove the old `EncryptedParams map[string]bool` after node-side decryption is gone.
- Modify `vmm/vmm.go`: add `tagDecryptKey interface{}` to `Vmm` and `New`.
- Create `vmm/encrypted_tags.go`: centralize VMM decryption helpers.
- Modify `vmm/spawn.go`, `vmm/apply.go`, `vmm/manage.go`: derive decrypted params at VMM boundaries and redact logs.
- Modify `node/node.go`, `node/handle.go`, `node/message.go`, `node/spawn.go`, `node/checkpoint.go`: pass raw data through and stop calling node-side decryption/sanitization.
- Delete `node/encrypted_tags.go`: remove node-side decryption helpers after callers are gone.
- Replace `node/encrypted_tags_test.go` with raw pass-through and checkpoint tests.
- Modify `vmm/spawn_test.go`, `vmm/apply_test.go`, and add `vmm/encrypted_tags_test.go`: cover VMM-derived decrypted params.
- Modify `vmm/core/token/handle.go`, `vmm/core/token/handle_test.go`: use `DecryptedParams` semantics and keep decrypted values out of forwarded tags.
- Modify `sdk/encrypted_tags_test.go`: update prefix expectations and decrypt assertions to `DecryptParams`.

---

### Task 1: Update tagcrypto Prefix and Decrypted Params Helpers

**Files:**
- Modify: `utils/tagcrypto/tagcrypto.go`
- Modify: `utils/tagcrypto/tagcrypto_test.go`

- [ ] **Step 1: Write failing tagcrypto tests for the new prefix and decrypted params map**

Add tests to `utils/tagcrypto/tagcrypto_test.go` near the existing round-trip tests:

```go
func TestDecryptParamsKeepsEncryptedTagKeys(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Secret", Value: "private-value"}}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "Encrypted-Secret", encrypted[0].Name)

	params, changed, err := DecryptParams(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, params)
}

func TestDecryptParamMapKeepsEncryptedTagKeys(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	encrypted, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)

	decrypted, changed, err := DecryptParamMap(map[string]string{
		"Public":           "public-value",
		encrypted[0].Name:  encrypted[0].Value,
	}, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decrypted)
}
```

- [ ] **Step 2: Run the focused tagcrypto tests and verify they fail**

Run: `go test ./utils/tagcrypto -run 'TestDecryptParamsKeepsEncryptedTagKeys|TestDecryptParamMapKeepsEncryptedTagKeys' -count=1`

Expected: FAIL because `EncryptedTagPrefix` is still `Hymx-Encrypted-`, and `DecryptParams` / `DecryptParamMap` do not exist.

- [ ] **Step 3: Implement the prefix change and helpers**

In `utils/tagcrypto/tagcrypto.go`, change the prefix constant:

```go
const (
	EncryptedTagPrefix    = "Encrypted-"
	CipherValuePrefix     = "hymxenc:v1"
	KeyTypeEthereumECIES  = "ethereum-ecies"
	KeyTypeArweaveRSAOAEP = "arweave-rsa-oaep-sha256"
)
```

Add these helpers below `DecryptTags`:

```go
func DecryptParams(tags []goarSchema.Tag, decryptKey interface{}) (map[string]string, bool, error) {
	decrypted := map[string]string{}
	changed := false
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			continue
		}
		if _, err := PlainTagName(tag.Name); err != nil {
			return nil, false, err
		}
		keyType, ciphertext, err := parseCipherValue(tag.Value)
		if err != nil {
			return nil, false, err
		}
		plaintext, err := decryptValue(ciphertext, keyType, decryptKey)
		if err != nil {
			return nil, false, err
		}
		decrypted[tag.Name] = string(plaintext)
		changed = true
	}
	return decrypted, changed, nil
}

func DecryptParamMap(params map[string]string, decryptKey interface{}) (map[string]string, bool, error) {
	tags := make([]goarSchema.Tag, 0, len(params))
	for key, value := range params {
		tags = append(tags, goarSchema.Tag{Name: key, Value: value})
	}
	return DecryptParams(tags, decryptKey)
}
```

Leave `DecryptTags` in place until all tests and call sites are migrated; it can be removed or kept as unused API after the refactor compiles.

- [ ] **Step 4: Update old tagcrypto round-trip expectations**

For tests that call `DecryptTags`, change assertions that expect `Secret` to use `DecryptParams` instead:

```go
decryptedParams, changed, err := DecryptParams(encrypted, nodeSigner)
require.NoError(t, err)
require.True(t, changed)
require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decryptedParams)
```

Keep malformed value, unsupported key type, wrong signer type, and reserved-name tests, but switch their function calls from `DecryptTags` to `DecryptParams` where the test is about execution decryption.

- [ ] **Step 5: Run tagcrypto tests and commit**

Run: `go test ./utils/tagcrypto -count=1`

Expected: PASS.

Commit:

```bash
git add utils/tagcrypto/tagcrypto.go utils/tagcrypto/tagcrypto_test.go
git commit -m "refactor: derive encrypted tag params without renaming"
```

---

### Task 2: Add Meta DecryptedParams and VMM Decryption Dependency

**Files:**
- Modify: `vmm/schema/schema.go`
- Modify: `vmm/vmm.go`
- Modify: `node/node.go`
- Modify: `vmm/spawn_test.go`
- Modify: `vmm/core/token/handle_test.go`
- Modify: `node/encrypted_tags_test.go`

- [ ] **Step 1: Add the new schema field while keeping the old field temporarily**

In `vmm/schema/schema.go`, change:

```go
Params          map[string]string `json:"Params"`
EncryptedParams map[string]bool   `json:"-"`
Data            string            `json:"Data"`
```

with:

```go
Params          map[string]string `json:"Params"`
EncryptedParams map[string]bool   `json:"-"`
DecryptedParams map[string]string `json:"-"`
Data            string            `json:"Data"`
```

- [ ] **Step 2: Run compilation and record expected failures**

Run: `go test ./vmm/... ./node/... -run TestDoesNotExist -count=0`

Expected: FAIL with references to old `vmm.New` signatures.

- [ ] **Step 3: Update VMM constructor and node wiring**

In `vmm/vmm.go`, add the field:

```go
	tagDecryptKey interface{}
```

Update `New`:

```go
func New(info *nodeSchema.Info, resultChan chan<- schema.VmmResult, outboxChan chan<- schema.Outbox, registrySpawned chan struct{}, tagDecryptKey interface{}) *Vmm {
	ctx, cancel := context.WithCancel(context.Background())
	return &Vmm{
		info:          info,
		tagDecryptKey: tagDecryptKey,

		vmFactors: map[string]schema.VmSpawnFunc{},
		vms:       map[string]schema.Vm{},
		vmsEnv:    map[string]*schema.Env{},

		vmsRecoveryLock: map[string]bool{},

		ctx:    ctx,
		cancel: cancel,

		resultChan:      resultChan,
		outboxChan:      outboxChan,
		applyChan:       make(chan schema.Meta, 1000),
		ckpChan:         make(chan schema.Checkpoint, 100),
		registrySpawned: registrySpawned,
	}
}
```

In `node/node.go`, update VMM creation:

```go
vmm: vmm.New(nodeInfo, resultChan, outboxChan, registryCh, signer),
```

In tests that construct VMM directly and do not need decryption, pass `nil`:

```go
vmm.New(&nodeSchema.Info{}, make(chan vmmSchema.VmmResult), make(chan vmmSchema.Outbox), make(chan struct{}, 1), nil)
```

- [ ] **Step 4: Update test fixtures that can already use DecryptedParams**

Update test fixtures that are explicitly about decrypted execution state:

```go
DecryptedParams: map[string]string{"Encrypted-X-Secret": "private-value"},
```

Keep existing production `EncryptedParams` checks for this task. They are removed in Tasks 4 and 5 after node-side decryption has been deleted. For tests that only need the new field to exist, use empty maps:

```go
DecryptedParams: map[string]string{},
```

- [ ] **Step 5: Run compile check and commit**

Run: `go test ./vmm/... ./node/... -run TestDoesNotExist -count=0`

Expected: PASS compile.

Commit:

```bash
git add vmm/schema/schema.go vmm/vmm.go node/node.go vmm/spawn_test.go vmm/core/token/handle_test.go node/encrypted_tags_test.go
git commit -m "refactor: add decrypted encrypted tag metadata"
```

---

### Task 3: Move Decryption to VMM Boundaries

**Files:**
- Create: `vmm/encrypted_tags.go`
- Create: `vmm/encrypted_tags_test.go`
- Modify: `vmm/spawn.go`
- Modify: `vmm/apply.go`
- Modify: `vmm/manage.go`
- Modify: `vmm/apply_test.go`

- [ ] **Step 1: Write VMM helper tests**

Create `vmm/encrypted_tags_test.go`:

```go
package vmm

import (
	"encoding/json"
	"testing"

	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestDeriveMetaDecryptedParamsPreservesRawParams(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	encryptedTags, _, err := tagcrypto.EncryptTags([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, tagcrypto.KeyTypeEthereumECIES)
	require.NoError(t, err)

	tags := append([]goarSchema.Tag{{Name: "Plain", Value: "public-value"}}, encryptedTags...)
	params, err := utils.TagsToParams(tags)
	require.NoError(t, err)
	v := &Vmm{tagDecryptKey: nodeSigner}

	meta, err := v.withDecryptedParamsFromTags(schema.Meta{Params: params}, tags)
	require.NoError(t, err)
	require.Equal(t, params, meta.Params)
	require.NotEqual(t, "private-value", meta.Params[tagcrypto.EncryptedTagPrefix+"Secret"])
	require.Equal(t, "private-value", meta.DecryptedParams[tagcrypto.EncryptedTagPrefix+"Secret"])
}

func TestDecryptedParamsAreNotSerializedInCheckpointEnv(t *testing.T) {
	env := schema.Env{
		Meta: schema.Meta{
			Pid: "process-id",
			Params: map[string]string{
				"Encrypted-Secret": "hymxenc:v1:ethereum-ecies:ciphertext",
			},
			DecryptedParams: map[string]string{"Encrypted-Secret": "private-value"},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Encrypted-Secret", Value: "hymxenc:v1:ethereum-ecies:ciphertext"}},
		},
	}
	by, err := json.Marshal(env)
	require.NoError(t, err)
	require.NotContains(t, string(by), "private-value")
	require.Contains(t, string(by), "Encrypted-Secret")
}
```

- [ ] **Step 2: Run VMM helper tests and verify failure**

Run: `go test ./vmm -run 'TestDeriveMetaDecryptedParamsPreservesRawParams|TestDecryptedParamsAreNotSerializedInCheckpointEnv' -count=1`

Expected: FAIL because `withDecryptedParamsFromTags` does not exist.

- [ ] **Step 3: Implement VMM decryption helpers**

Create `vmm/encrypted_tags.go`:

```go
package vmm

import (
	"github.com/hymatrix/hymx/utils/tagcrypto"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (v *Vmm) withDecryptedParamsFromTags(meta schema.Meta, tags []goarSchema.Tag) (schema.Meta, error) {
	decrypted, _, err := tagcrypto.DecryptParams(tags, v.tagDecryptKey)
	if err != nil {
		return meta, err
	}
	meta.DecryptedParams = decrypted
	return meta, nil
}

func (v *Vmm) withDecryptedParamsFromParamMap(meta schema.Meta) (schema.Meta, error) {
	decrypted, _, err := tagcrypto.DecryptParamMap(meta.Params, v.tagDecryptKey)
	if err != nil {
		return meta, err
	}
	meta.DecryptedParams = decrypted
	return meta, nil
}
```

- [ ] **Step 4: Derive decrypted params in Spawn**

In `vmm/spawn.go`, at the start of `Spawn` after `pid := meta.Pid`, add:

```go
	meta, err = v.withDecryptedParamsFromTags(meta, process.Tags)
	if err != nil {
		return err
	}
```

This uses the existing named return `err error`.

- [ ] **Step 5: Derive decrypted params in Apply and redact both maps**

In `vmm/apply.go`, at the start of `apply` before `GetVm`:

```go
	meta, err := v.withDecryptedParamsFromParamMap(meta)
	if err != nil {
		return err
	}
```

Update `redactedMeta`:

```go
func redactedMeta(meta schema.Meta) schema.Meta {
	if len(meta.Params) != 0 {
		params := make(map[string]string, len(meta.Params))
		for key := range meta.Params {
			params[key] = "[redacted]"
		}
		meta.Params = params
	}
	if len(meta.DecryptedParams) != 0 {
		decryptedParams := make(map[string]string, len(meta.DecryptedParams))
		for key := range meta.DecryptedParams {
			decryptedParams[key] = "[redacted]"
		}
		meta.DecryptedParams = decryptedParams
	}
	return meta
}
```

- [ ] **Step 6: Derive decrypted params in Restore before spawn/addVm**

In `vmm/manage.go`, update `Restore` before `GetVm`:

```go
	meta, err := v.withDecryptedParamsFromTags(snap.Env.Meta, snap.Env.Process.Tags)
	if err != nil {
		return err
	}
	snap.Env.Meta = meta
```

Keep the rest of `Restore` unchanged so restored `vmsEnv` stores raw params plus non-serialized decrypted params.

- [ ] **Step 7: Run VMM tests and commit**

Run: `go test ./vmm -count=1`

Expected: PASS.

Commit:

```bash
git add vmm/encrypted_tags.go vmm/encrypted_tags_test.go vmm/spawn.go vmm/apply.go vmm/manage.go vmm/apply_test.go
git commit -m "refactor: decrypt encrypted tags at vmm boundary"
```

---

### Task 4: Remove Node-Side Decryption and Checkpoint Sanitization

**Files:**
- Modify: `node/handle.go`
- Modify: `node/message.go`
- Modify: `node/spawn.go`
- Modify: `node/checkpoint.go`
- Delete: `node/encrypted_tags.go`
- Rewrite: `node/encrypted_tags_test.go`

- [ ] **Step 1: Replace node encrypted tag tests with raw pass-through tests**

Replace the old contents of `node/encrypted_tags_test.go` with tests that prove node no longer decrypts:

```go
package node

import (
	"encoding/json"
	"testing"

	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestNodeDecodeKeepsEncryptedTagsRaw(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	encryptedTags, _, err := tagcrypto.EncryptTags([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, tagcrypto.KeyTypeEthereumECIES)
	require.NoError(t, err)

	rawTags := utils.MergeTags([]goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
	}, encryptedTags)
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "process-id", "", rawTags)
	require.NoError(t, err)

	_, _, _, instance, err := utils.Decode(rawItem)
	require.NoError(t, err)
	msg := instance.(hymxSchema.Message)
	require.Empty(t, utils.GetTagsValue("Secret", msg.Tags))
	require.NotEmpty(t, utils.GetTagsValue(tagcrypto.EncryptedTagPrefix+"Secret", msg.Tags))
}

func TestCheckpointSnapshotKeepsRawEncryptedEnv(t *testing.T) {
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "process-id",
				Params: map[string]string{
					"Encrypted-Secret": "hymxenc:v1:ethereum-ecies:ciphertext",
				},
				DecryptedParams: map[string]string{"Encrypted-Secret": "private-value"},
			},
			Process: hymxSchema.Process{
				Tags: []goarSchema.Tag{{Name: "Encrypted-Secret", Value: "hymxenc:v1:ethereum-ecies:ciphertext"}},
			},
		},
		Data: "vm-state",
	}
	by, err := json.Marshal(snap)
	require.NoError(t, err)
	require.Contains(t, string(by), "Encrypted-Secret")
	require.Contains(t, string(by), "ciphertext")
	require.NotContains(t, string(by), "private-value")
}
```

- [ ] **Step 2: Run node encrypted tag tests and verify old implementation fails**

Run: `go test ./node -run 'TestNodeDecodeKeepsEncryptedTagsRaw|TestCheckpointSnapshotKeepsRawEncryptedEnv' -count=1`

Expected: PASS for raw decode if tests use `utils.Decode`, but the package should still fail overall until old node-side decryption call sites and tests are removed.

- [ ] **Step 3: Update Handle and HandleMode to decode raw items**

In `node/handle.go`, replace:

```go
_, internalInstance, err := n.decryptInternalItem(item)
if err != nil {
	return
}
```

with:

```go
internalInstance := instance
```

In `HandleMode`, replace:

```go
pid, accid, _, instance, err := n.decodeInternalItem(item)
```

with:

```go
pid, accid, _, instance, err := utils.Decode(item)
```

- [ ] **Step 4: Stop computing encrypted flags in node apply paths**

In `node/message.go`, remove the `tagcrypto` import and delete:

```go
encryptedParams, err := tagcrypto.EncryptedPlainTagNames(item.Tags)
if err != nil {
	return err
}
```

Remove `EncryptedParams: encryptedParams,` from `vmmSchema.Meta`.

In `node/spawn.go`, remove the `tagcrypto` import and delete:

```go
encryptedParams, err := tagcrypto.EncryptedPlainTagNames(item.Tags)
if err != nil {
	return err
}
```

Remove `EncryptedParams: encryptedParams,` from `vmmSchema.Meta`.

In `vmm/schema/schema.go`, remove the old field:

```go
EncryptedParams map[string]bool   `json:"-"`
```

- [ ] **Step 5: Remove checkpoint sanitize/decrypt calls**

In `node/checkpoint.go`, remove from `Restore`:

```go
snap, err = n.decryptSnapshotEnv(snap)
if err != nil {
	return -1, err
}
```

In `signCheckpoint`, remove:

```go
snap, err = n.sanitizeCheckpointSnapshot(snap)
if err != nil {
	return
}
```

- [ ] **Step 6: Delete node encrypted tag helper file**

Delete `node/encrypted_tags.go` after all callers are removed.

- [ ] **Step 7: Run node tests and commit**

Run: `go test ./node -count=1`

Expected: PASS.

Commit:

```bash
git add node/handle.go node/message.go node/spawn.go node/checkpoint.go node/encrypted_tags_test.go vmm/schema/schema.go
git rm node/encrypted_tags.go
git commit -m "refactor: pass encrypted tags through node raw"
```

---

### Task 5: Update Core Module Forwarding Semantics

**Files:**
- Modify: `vmm/spawn.go`
- Modify: `vmm/spawn_test.go`
- Modify: `vmm/core/token/handle.go`
- Modify: `vmm/core/token/handle_test.go`

- [ ] **Step 1: Update spawn forwarding tests**

In `vmm/spawn_test.go`, replace encrypted-origin tests with raw encrypted-key tests:

```go
func TestGenSpawnResultDoesNotForwardDecryptedParams(t *testing.T) {
	env := &schema.Env{
		Meta: schema.Meta{
			Pid:         "process-id",
			FromProcess: "from-process",
			Params: map[string]string{
				"X-Public":           "public-value",
				"Encrypted-X-Secret": "ciphertext",
			},
			DecryptedParams: map[string]string{"Encrypted-X-Secret": "private-value"},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{
				{Name: "Reference", Value: "7"},
				{Name: "Encrypted-X-Secret", Value: "ciphertext"},
			},
		},
	}
	v := &Vmm{}

	result := v.genSpawnResult(env)

	tags := result.Messages[0].Tags
	require.Equal(t, "public-value", tagValue(tags, "X-Public"))
	require.Empty(t, tagValue(tags, "Encrypted-X-Secret"))
	require.NotContains(t, tags, goarSchema.Tag{Name: "Encrypted-X-Secret", Value: "private-value"})
}
```

- [ ] **Step 2: Update spawn forwarding implementation**

In `vmm/spawn.go`, replace `EncryptedParams` checks with prefix-safe logic:

```go
ref := utils.GetTagsValueByDefault("Reference", env.Process.Tags, "0")
```

Keep X-tag forwarding as:

```go
for key, value := range env.Meta.Params {
	if strings.HasPrefix(key, "X-") {
		spawnedMsg.Tags = append(spawnedMsg.Tags, goarSchema.Tag{Name: key, Value: value})
	}
}
```

This does not forward decrypted values because encrypted keys are `Encrypted-*`, not `X-*`.

- [ ] **Step 3: Update token forwarding tests**

In `vmm/core/token/handle_test.go`, update the transfer test meta:

```go
Params: map[string]string{
	"Recipient":           recipient,
	"Quantity":            "100",
	"X-Public":            "public-value",
	"Encrypted-X-Secret":  "ciphertext",
},
DecryptedParams: map[string]string{"Encrypted-X-Secret": "private-value"},
```

Assert:

```go
require.Equal(t, "public-value", tokenTagValue(msg.Tags, "X-Public"))
require.Empty(t, tokenTagValue(msg.Tags, "Encrypted-X-Secret"))
require.NotContains(t, msg.Tags, goarSchema.Tag{Name: "Encrypted-X-Secret", Value: "private-value"})
```

- [ ] **Step 4: Update token forwarding implementation**

In `vmm/core/token/handle.go`, replace:

```go
if strings.HasPrefix(key, "X-") && !meta.EncryptedParams[key] {
```

with:

```go
if strings.HasPrefix(key, "X-") {
```

Because encrypted values are no longer present under stripped `X-*` keys.

- [ ] **Step 5: Run VMM and token tests and commit**

Run: `go test ./vmm ./vmm/core/token -count=1`

Expected: PASS.

Commit:

```bash
git add vmm/spawn.go vmm/spawn_test.go vmm/core/token/handle.go vmm/core/token/handle_test.go
git commit -m "refactor: keep decrypted params out of forwarded tags"
```

---

### Task 6: Update SDK and Integration Tests for New Prefix Semantics

**Files:**
- Modify: `sdk/encrypted_tags_test.go`
- Modify: `sdk/sdk_test.go` if prefix literals appear after search
- Modify: `node/encrypted_tags_test.go` if integration expectations need VMM error assertions
- Modify: `utils/tagcrypto/tagcrypto_test.go` if old `DecryptTags` assertions remain

- [ ] **Step 1: Search for old prefix and old field references**

Run: `rg -n "Hymx-Encrypted-|EncryptedParams|DecryptTags\\(" .`

Expected: Results only in compatibility notes or old tests before this task starts. After this task, there should be no production references to `EncryptedParams` and no tests expecting stripped decrypted keys.

- [ ] **Step 2: Update SDK tests to assert `Encrypted-` prefix**

In `sdk/encrypted_tags_test.go`, replace direct decrypted tag-list checks:

```go
decrypted, changed, err := tagcrypto.DecryptTags(submitted.Tags, nodeSigner)
require.NoError(t, err)
require.True(t, changed)
require.Equal(t, "private-value", tagValue(decrypted, "Secret"))
```

with decrypted-param checks:

```go
decryptedParams, changed, err := tagcrypto.DecryptParams(submitted.Tags, nodeSigner)
require.NoError(t, err)
require.True(t, changed)
require.Equal(t, "private-value", decryptedParams[tagcrypto.EncryptedTagPrefix+"Secret"])
```

Keep ciphertext assertions:

```go
require.Equal(t, tagcrypto.EncryptedTagPrefix+"Secret", tagValueName(submitted.Tags, tagcrypto.EncryptedTagPrefix+"Secret"))
ciphertext := tagValue(submitted.Tags, tagcrypto.EncryptedTagPrefix+"Secret")
require.NotEmpty(t, ciphertext)
require.NotContains(t, ciphertext, "private-value")
```

- [ ] **Step 3: Update redirect decrypt assertions**

For tests that assert the first or second node cannot decrypt redirected ciphertext, replace:

```go
_, _, err = tagcrypto.DecryptTags(redirectedItem.Tags, firstNodeSigner)
require.Error(t, err)
```

with:

```go
_, _, err = tagcrypto.DecryptParams(redirectedItem.Tags, firstNodeSigner)
require.Error(t, err)
```

For the target node success path:

```go
decryptedParams, changed, err := tagcrypto.DecryptParams(redirectedItem.Tags, secondNodeSigner)
require.NoError(t, err)
require.True(t, changed)
require.Equal(t, "private-value", decryptedParams[tagcrypto.EncryptedTagPrefix+"Secret"])
```

- [ ] **Step 4: Run SDK encrypted tests**

Run: `go test ./sdk -run Encrypted -count=1`

Expected: PASS.

- [ ] **Step 5: Run repository-wide reference check**

Run: `rg -n "Hymx-Encrypted-|EncryptedParams" .`

Expected: no matches in `.go` files. Matches are acceptable only in committed design documents that mention the old prefix as migration context.

- [ ] **Step 6: Commit SDK and test updates**

Commit:

```bash
git add sdk/encrypted_tags_test.go sdk/sdk_test.go node/encrypted_tags_test.go utils/tagcrypto/tagcrypto_test.go
git commit -m "test: update encrypted tag prefix expectations"
```

---

### Task 7: End-to-End Verification

**Files:**
- Verify all modified files from Tasks 1-6.

- [ ] **Step 1: Run full Go test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run final leak/reference checks**

Run:

```bash
rg -n "private-value" ckp db node vmm sdk utils
rg -n "EncryptedParams|Hymx-Encrypted-" --glob '*.go' .
```

Expected: first command may find test fixtures only; no production code should embed decrypted checkpoint values. Second command should return no `.go` matches.

- [ ] **Step 3: Inspect git diff for accidental unrelated changes**

Run: `git diff --stat HEAD~6..HEAD`

Expected: changes are limited to tagcrypto, VMM, node, SDK tests, and token forwarding files described in this plan.

- [ ] **Step 4: Commit final cleanup when verification changed files**

Run: `git status --short`

Expected when no cleanup was needed: no modified tracked files.

If cleanup changed tracked files, inspect them with `git diff`, stage the exact files shown by `git status --short`, and commit with `git commit -m "chore: verify encrypted tag vmm refactor"`.

If no cleanup was needed, do not create an empty commit.

---

## Self-Review

- Spec coverage: Tasks 1-6 cover the new prefix, raw node pass-through, VMM-owned decryption, `DecryptedParams`, checkpoint raw serialization, restore derivation, reserved-tag errors, and SDK test updates.
- Placeholder scan: The plan contains exact files, commands, expected results, and code snippets for each code-changing step.
- Type consistency: The plan consistently uses `DecryptedParams map[string]string`, `tagDecryptKey interface{}`, `DecryptParams`, and `DecryptParamMap`.
