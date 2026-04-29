package sdk

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/everFinance/goether"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/require"
)

func TestSendEncryptsPrefixedCustomTagBeforeSigning(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	var submitted goarSchema.BundleItem
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			submitted = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(server.URL, userBundler)

	_, _, err = s.Send("lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})
	require.NoError(t, err)

	require.NotEmpty(t, submitted.Id)
	require.Equal(t, tagcrypto.EncryptedTagPrefix+"Secret", tagValueName(submitted.Tags, tagcrypto.EncryptedTagPrefix+"Secret"))
	ciphertext := tagValue(submitted.Tags, tagcrypto.EncryptedTagPrefix+"Secret")
	require.NotContains(t, ciphertext, "private-value")
	require.True(t, strings.HasPrefix(ciphertext, tagcrypto.CipherValuePrefix+":"+tagcrypto.KeyTypeEthereumECIES+":"))

	decrypted, changed, err := tagcrypto.DecryptTags(submitted.Tags, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "private-value", tagValue(decrypted, "Secret"))
}

func TestSendAcceptsSingleNodeEncryptedSpawnRedirect(t *testing.T) {
	firstNodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	firstNodeBundler, err := goar.NewBundler(firstNodeSigner)
	require.NoError(t, err)
	secondNodeSigner, err := goether.NewSigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	secondNodeBundler, err := goar.NewBundler(secondNodeSigner)
	require.NoError(t, err)

	var redirectedItem goarSchema.BundleItem
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: secondNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			redirectedItem = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer redirected.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: firstNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode(registrySchema.Node{URL: redirected.URL})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(primary.URL, userBundler)

	_, redirectedURL, err := s.Send("", "payload", []goarSchema.Tag{
		{Name: "Data-Protocol", Value: schema.DataProtocol},
		{Name: "Variant", Value: schema.Variant},
		{Name: "Type", Value: schema.TypeProcess},
		{Name: "Module", Value: "module-id"},
		{Name: "Scheduler", Value: secondNodeBundler.Address},
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"},
	})
	require.NoError(t, err)
	require.Equal(t, redirected.URL, redirectedURL)

	decrypted, changed, err := tagcrypto.DecryptTags(redirectedItem.Tags, secondNodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "private-value", tagValue(decrypted, "Secret"))
}

func TestSpawnAndWaitPollsRedirectedNodeForEncryptedSpawn(t *testing.T) {
	firstNodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	firstNodeBundler, err := goar.NewBundler(firstNodeSigner)
	require.NoError(t, err)
	secondNodeSigner, err := goether.NewSigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	secondNodeBundler, err := goar.NewBundler(secondNodeSigner)
	require.NoError(t, err)

	var redirectedItem goarSchema.BundleItem
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: secondNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			redirectedItem = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			if strings.HasPrefix(r.URL.Path, "/result/") {
				json.NewEncoder(w).Encode(vmmSchema.VmmResult{ItemId: redirectedItem.Id})
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer redirected.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: firstNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: redirected.URL}})
		default:
			if strings.HasPrefix(r.URL.Path, "/result/") {
				http.Error(w, "wrong node", http.StatusInternalServerError)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(primary.URL, userBundler)

	res, err := s.SpawnAndWait("module-id", secondNodeBundler.Address, []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})
	require.NoError(t, err)
	require.Equal(t, redirectedItem.Id, res.Id)
}

func TestSendReencryptsEncryptedTagsForRedirectedNode(t *testing.T) {
	firstNodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	firstNodeBundler, err := goar.NewBundler(firstNodeSigner)
	require.NoError(t, err)
	secondNodeSigner, err := goether.NewSigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	secondNodeBundler, err := goar.NewBundler(secondNodeSigner)
	require.NoError(t, err)

	var redirectedItem goarSchema.BundleItem
	redirected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: secondNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			redirectedItem = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer redirected.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: firstNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: redirected.URL}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(primary.URL, userBundler)

	_, redirectedURL, err := s.Send("lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})
	require.NoError(t, err)
	require.NotEmpty(t, redirectedURL)

	_, _, err = tagcrypto.DecryptTags(redirectedItem.Tags, firstNodeSigner)
	require.Error(t, err)
	decrypted, changed, err := tagcrypto.DecryptTags(redirectedItem.Tags, secondNodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "private-value", tagValue(decrypted, "Secret"))
}

func TestSendFollowsChainedEncryptedRedirects(t *testing.T) {
	firstNodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	firstNodeBundler, err := goar.NewBundler(firstNodeSigner)
	require.NoError(t, err)
	secondNodeSigner, err := goether.NewSigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	secondNodeBundler, err := goar.NewBundler(secondNodeSigner)
	require.NoError(t, err)
	thirdNodeSigner, err := goether.NewSigner("0x2222222222222222222222222222222222222222222222222222222222222222")
	require.NoError(t, err)
	thirdNodeBundler, err := goar.NewBundler(thirdNodeSigner)
	require.NoError(t, err)

	var finalItem goarSchema.BundleItem
	finalNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: thirdNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			finalItem = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer finalNode.Close()

	middleNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: secondNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: finalNode.URL}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer middleNode.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: firstNodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: middleNode.URL}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(primary.URL, userBundler)

	_, redirectedURL, err := s.Send("lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})
	require.NoError(t, err)
	require.Equal(t, finalNode.URL, redirectedURL)

	_, _, err = tagcrypto.DecryptTags(finalItem.Tags, firstNodeSigner)
	require.Error(t, err)
	_, _, err = tagcrypto.DecryptTags(finalItem.Tags, secondNodeSigner)
	require.Error(t, err)
	decrypted, changed, err := tagcrypto.DecryptTags(finalItem.Tags, thirdNodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "private-value", tagValue(decrypted, "Secret"))
}

func TestSendReturnsErrorForEncryptedRedirectWithoutUsableNodes(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode(registrySchema.Node{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler(primary.URL, userBundler)

	res, redirectedURL, err := s.Send("lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})
	require.Error(t, err)
	require.Nil(t, res)
	require.Empty(t, redirectedURL)
}

func newEncryptedTagTestSDK(t *testing.T, baseURL string) *SDK {
	t.Helper()

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	return NewFromBundler(baseURL, userBundler)
}

func tagValue(tags []goarSchema.Tag, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Value
		}
	}
	return ""
}

func tagValueName(tags []goarSchema.Tag, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Name
		}
	}
	return ""
}

func TestSendRejectsEncryptedProtocolTag(t *testing.T) {
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	s := NewFromBundler("http://127.0.0.1:1", userBundler)

	_, _, err = s.Send("", "", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Type", Value: schema.TypeMessage}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
}

func TestSendEncryptedTagRequiresInfoPublicKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionKeyType: tagcrypto.KeyTypeEthereumECIES,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "does not advertise encryption metadata")
}

func TestSendEncryptedTagRequiresInfoKeyType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err = s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "does not advertise encryption metadata")
}

func TestSendPlainTagsDoesNotFetchInfo(t *testing.T) {
	infoRequests := 0
	var submitted goarSchema.BundleItem
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			infoRequests++
			http.Error(w, "info should not be fetched", http.StatusInternalServerError)
		case "/":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			item, err := goarUtils.DecodeBundleItem(body)
			require.NoError(t, err)
			submitted = item
			json.NewEncoder(w).Encode(map[string]string{"Id": item.Id})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "payload", []goarSchema.Tag{{Name: "Plain", Value: "public-value"}})

	require.NoError(t, err)
	require.Zero(t, infoRequests)
	require.Equal(t, "public-value", tagValue(submitted.Tags, "Plain"))
}

func TestSendEncryptedRedirectReturnsErrorWhenRedirectInfoIsUnusable(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	badRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer badRedirect.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			json.NewEncoder(w).Encode(nodeSchema.Info{
				EncryptionPublicKey: nodeBundler.Owner,
				EncryptionKeyType:   tagcrypto.KeyTypeEthereumECIES,
			})
		case "/":
			w.WriteHeader(http.StatusPermanentRedirect)
			json.NewEncoder(w).Encode([]registrySchema.Node{{URL: badRedirect.URL}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer primary.Close()

	s := newEncryptedTagTestSDK(t, primary.URL)

	res, redirectedURL, err := s.Send("process-id", "payload", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}})

	require.Error(t, err)
	require.Nil(t, res)
	require.Empty(t, redirectedURL)
}

func TestSendRejectsEncryptedProtocolTagBeforeNetworkAccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "network should not be reached", http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newEncryptedTagTestSDK(t, server.URL)

	_, _, err := s.Send("", "", []goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Type", Value: schema.TypeMessage}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
	require.Zero(t, requests)
}
