package vmm

import (
	"bytes"
	"errors"
	"testing"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/inconshreveable/log15"
	"github.com/stretchr/testify/require"
)

func TestSafeLogMetaRedactsParams(t *testing.T) {
	var buf bytes.Buffer
	oldLog := log
	log = common.NewLog("vmm-test")
	log.SetHandler(log15.StreamHandler(&buf, log15.LogfmtFormat()))
	defer func() { log = oldLog }()

	logApplyStart(schema.Meta{
		ItemId: "item-id",
		Pid:    "pid",
		Params: map[string]string{
			"Secret": "private-value",
		},
		DecryptedParams: map[string]string{
			"Encrypted-Secret": "decrypted-private-value",
		},
	}, nil)

	require.NotContains(t, buf.String(), "private-value")
	require.NotContains(t, buf.String(), "decrypted-private-value")
	require.Contains(t, buf.String(), "Secret")
	require.Contains(t, buf.String(), "Encrypted-Secret")
}

func TestApplyDecryptFailureEmitsResultAndUnlocksRecovery(t *testing.T) {
	resultChan := make(chan schema.VmmResult, 1)
	v := New(nil, resultChan, make(chan schema.Outbox, 1), make(chan struct{}), nil)
	env := &schema.Env{
		Meta:  schema.Meta{Pid: "process-id"},
		Nonce: 4,
	}
	v.addVm(testVm{}, env)
	v.RecoveryLock("process-id")

	err := v.apply(schema.Meta{
		Pid:              "process-id",
		ItemId:           "item-id",
		Nonce:            5,
		RecoveryMaxNonce: 5,
		Mode:             schema.ExecModeReplay,
		Params: map[string]string{
			"Encrypted-Secret": "not-a-cipher",
		},
	})

	require.Error(t, err)
	require.False(t, v.IsRecovering("process-id"))
	result := <-resultChan
	require.Equal(t, "item-id", result.ItemId)
	require.Equal(t, "process-id", result.FromProcess)
	require.Contains(t, result.Error, "invalid encrypted tag value")
}

type testVm struct{}

func (testVm) Apply(string, schema.Meta) schema.Result {
	return schema.Result{Error: errors.New("apply should not be called")}
}

func (testVm) Checkpoint() (string, error) {
	return "", nil
}

func (testVm) Restore(string) error {
	return nil
}

func (testVm) Close() error {
	return nil
}
