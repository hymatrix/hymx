package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryQueriesVerifyResponses(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	msg, err := bundler.CreateAndSignItem([]byte("data"), "", "", nil)
	require.NoError(t, err)
	tags, err := utils.AssignmentToTags(schema.Assignment{Base: schema.DefaultBaseAssignment, Process: "pid", Message: msg.Id, Nonce: "1", Timestamp: "1"})
	require.NoError(t, err)
	assign, err := bundler.CreateAndSignItem(nil, "", "", tags)
	require.NoError(t, err)
	for _, tc := range []struct {
		path string
		item goarSchema.BundleItem
		get  func(*Client) (goarSchema.BundleItem, error)
	}{
		{path: "/messageBin/" + msg.Id, item: msg, get: func(c *Client) (goarSchema.BundleItem, error) { return c.GetMessageBin(msg.Id) }},
		{path: "/messageBinByNonce/pid/1", item: msg, get: func(c *Client) (goarSchema.BundleItem, error) { return c.GetMessageBinByNonce("pid", 1) }},
		{path: "/assignmentBinByNonce/pid/1", item: assign, get: func(c *Client) (goarSchema.BundleItem, error) { return c.GetAssignBinByNonce("pid", 1) }},
		{path: "/assignmentBinByMessage/" + msg.Id, item: assign, get: func(c *Client) (goarSchema.BundleItem, error) { return c.GetAssignBinByMessage(msg.Id) }},
	} {
		t.Run(tc.path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.path, r.URL.Path)
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write(tc.item.Binary)
			}))
			t.Cleanup(server.Close)
			got, err := tc.get(NewClient(server.URL))
			require.NoError(t, err)
			assert.Equal(t, tc.item.Id, got.Id)
			assert.ElementsMatch(t, tc.item.Tags, got.Tags)
		})
	}
	badSignature := append([]byte{}, msg.Binary...)
	badSignature[2] ^= 1
	for _, tc := range []struct {
		name, contentType string
		status            int
		body              []byte
	}{
		{name: "JSON", contentType: "application/json", status: 200, body: []byte(`{}`)},
		{name: "missing", contentType: "application/octet-stream", status: 404},
		{name: "truncated", contentType: "application/octet-stream", status: 200, body: msg.Binary[:20]},
		{name: "signature", contentType: "application/octet-stream", status: 200, body: badSignature},
		{name: "wrong id", contentType: "application/octet-stream", status: 200, body: assign.Binary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			t.Cleanup(server.Close)
			_, err := NewClient(server.URL).GetMessageBin(msg.Id)
			assert.Error(t, err)
		})
	}
}

func TestDecodeItemBinaryRejectsTruncationAndTampering(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem([]byte("data"), "", "", nil)
	require.NoError(t, err)
	got, err := decodeItemBinary(item.Binary)
	require.NoError(t, err)
	assert.Equal(t, item.Id, got.Id)
	for i := 0; i < len(item.Binary); i++ {
		got, err := decodeItemBinary(item.Binary[:i])
		require.Error(t, err, "prefix length %d", i)
		assert.Empty(t, got.Id)
	}
	changed := append([]byte{}, item.Binary...)
	changed[len(changed)-1] ^= 1
	_, err = decodeItemBinary(changed)
	assert.Error(t, err)
}
