package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hymatrix/hymx/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
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
	other, err := bundler.CreateAndSignItem([]byte("other"), "", "", nil)
	require.NoError(t, err)
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
		{name: "wrong id", contentType: "application/octet-stream", status: 200, body: other.Binary},
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

func TestBinaryQueriesFallBackToLegacyJSON(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	msg, err := bundler.CreateAndSignItem([]byte("data"), "", "", []goarSchema.Tag{{Name: "Scheduler", Value: "A"}})
	require.NoError(t, err)
	assign, err := bundler.CreateAndSignItem([]byte{}, "", "", []goarSchema.Tag{
		{Name: "Type", Value: schema.TypeAssignment}, {Name: "Message", Value: msg.Id},
	})
	require.NoError(t, err)
	// Legacy nodes serve JSON only; rewritten JSON tags must not reach callers.
	legacy := map[string]goarSchema.BundleItem{
		"/message/" + msg.Id:             withTags(msg, goarSchema.Tag{Name: "Scheduler", Value: "B"}),
		"/messageByNonce/pid/1":          withTags(msg, goarSchema.Tag{Name: "Scheduler", Value: "B"}),
		"/assignmentByNonce/pid/1":       withTags(assign, goarSchema.Tag{Name: "Message", Value: "forged"}),
		"/assignmentByMessage/" + msg.Id: withTags(assign, goarSchema.Tag{Name: "Message", Value: "forged"}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		item, ok := legacy[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(item))
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL)

	for _, tc := range []struct {
		name  string
		query func() (goarSchema.BundleItem, error)
		want  goarSchema.BundleItem
	}{
		{name: "message", query: func() (goarSchema.BundleItem, error) { return client.GetMessageBin(msg.Id) }, want: msg},
		{name: "message by nonce", query: func() (goarSchema.BundleItem, error) { return client.GetMessageBinByNonce("pid", 1) }, want: msg},
		{name: "assign by nonce", query: func() (goarSchema.BundleItem, error) { return client.GetAssignBinByNonce("pid", 1) }, want: assign},
		{name: "assign by message", query: func() (goarSchema.BundleItem, error) { return client.GetAssignBinByMessage(msg.Id) }, want: assign},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.query()
			require.NoError(t, err)
			assert.Equal(t, tc.want.Id, got.Id)
			assert.Equal(t, tc.want.Tags, got.Tags)
		})
	}
}

func TestLegacyJSONFallbackRejectsTampering(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	msg, err := bundler.CreateAndSignItem([]byte("data"), "", "", []goarSchema.Tag{{Name: "Scheduler", Value: "A"}})
	require.NoError(t, err)
	other, err := bundler.CreateAndSignItem([]byte("other"), "", "", []goarSchema.Tag{{Name: "Scheduler", Value: "A"}})
	require.NoError(t, err)
	forgedTagsBy, err := goarUtils.SerializeTags([]goarSchema.Tag{{Name: "Scheduler", Value: "B"}})
	require.NoError(t, err)

	tagsBy := msg
	tagsBy.Tags = []goarSchema.Tag{{Name: "Scheduler", Value: "B"}}
	tagsBy.TagsBy = goarUtils.Base64Encode(forgedTagsBy)
	noTags := msg
	noTags.Tags = nil
	signature := msg
	signature.Signature = other.Signature
	for _, tc := range []struct {
		name string
		body any
	}{
		{name: "tags bytes", body: tagsBy},
		{name: "tags removed", body: noTags},
		{name: "signature", body: signature},
		{name: "wrong id", body: other},
		{name: "null", body: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/message/"+msg.Id {
					http.NotFound(w, r)
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(tc.body))
			}))
			t.Cleanup(server.Close)
			got, err := NewClient(server.URL).GetMessageBin(msg.Id)
			assert.Error(t, err)
			assert.Empty(t, got.Id)
		})
	}
}

func withTags(item goarSchema.BundleItem, tags ...goarSchema.Tag) goarSchema.BundleItem {
	item.Tags = tags
	return item
}
