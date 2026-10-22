package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BinaryTestSuite struct{ suite.Suite }

func (suite *BinaryTestSuite) TestRebuildStoredItem() {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(suite.T(), err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(suite.T(), err)
	item, err := bundler.CreateAndSignItem([]byte("data"), "", "", []goarSchema.Tag{{Name: "Scheduler", Value: "A"}})
	require.NoError(suite.T(), err)
	encoded, err := json.Marshal(item)
	require.NoError(suite.T(), err)
	var stored goarSchema.BundleItem
	require.NoError(suite.T(), json.Unmarshal(encoded, &stored))
	require.Empty(suite.T(), stored.Binary)
	stored.Tags[0].Value = "B"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeItemBinary(c, &stored, nil)
	require.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/octet-stream", w.Header().Get("Content-Type"))
	got, err := goarUtils.DecodeBundleItem(w.Body.Bytes())
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), goarUtils.VerifyBundleItem(got))
	assert.Equal(suite.T(), item.Id, got.Id)
	assert.Equal(suite.T(), item.Tags, got.Tags)
	assert.Equal(suite.T(), item.Data, got.Data)
}

func (suite *BinaryTestSuite) TestInvalidNonce() {
	s := &Server{}
	engine := gin.New()
	engine.GET("/messageBinByNonce/:pid/:nonce", s.GetMessageBinByNonce)
	engine.GET("/assignmentBinByNonce/:pid/:nonce", s.GetAssignBinByNonce)
	for _, path := range []string{"/messageBinByNonce/pid/no", "/assignmentBinByNonce/pid/-1"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	}
}

func TestBinaryTestSuite(t *testing.T) { suite.Run(t, new(BinaryTestSuite)) }
