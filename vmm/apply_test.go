package vmm

import (
	"bytes"
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
	}, nil)

	require.NotContains(t, buf.String(), "private-value")
	require.Contains(t, buf.String(), "Secret")
}
