package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/permadao/goar"
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
