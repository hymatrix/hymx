package node

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hymatrix/hymx/sdk"
	"github.com/hymatrix/hymx/utils"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Upgraded senders must still commit outbox items hosted by nodes without binary routes.
func TestTryGetAssignFromLegacyNode(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	n := &Node{bundler: bundler, sdk: sdk.NewFromBundler("", bundler)}
	msg, err := bundler.CreateAndSignItem([]byte("data"), "", "", nil)
	require.NoError(t, err)
	_, assign, err := n.signAssign("target", msg.Id, 1)
	require.NoError(t, err)
	stored := assign
	stored.Tags = append([]goarSchema.Tag{}, assign.Tags...)
	for i := range stored.Tags {
		if stored.Tags[i].Name == "Message" {
			stored.Tags[i].Value = "forged"
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/":
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/assignmentByMessage/"+msg.Id:
			require.NoError(t, json.NewEncoder(w).Encode(stored))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	got, err := n.tryGetAssign(msg.Id, []registrySchema.Node{{AccId: bundler.Address, URL: server.URL}}, msg.Binary)
	require.NoError(t, err)
	assert.Equal(t, assign.Id, got.Id)
	assert.Equal(t, msg.Id, utils.GetTagsValue("Message", got.Tags))
}
