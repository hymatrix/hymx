package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/utils"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeItemBinary(c, &stored, nil)
	require.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Equal(suite.T(), "application/octet-stream", w.Header().Get("Content-Type"))
	got, err := utils.DecodeBundleItem(w.Body.Bytes())
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), item.Id, got.Id)
	assert.Equal(suite.T(), item.Tags, got.Tags)
	assert.Equal(suite.T(), item.Data, got.Data)
	stored.Tags[0].Value = "B"
	for _, tc := range []struct {
		name   string
		item   *goarSchema.BundleItem
		err    error
		status int
	}{
		{name: "tampered", item: &stored, status: 400},
		{name: "missing", status: 404},
		{name: "database error", err: errors.New("database error"), status: 400},
	} {
		suite.Run(tc.name, func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			writeItemBinary(c, tc.item, tc.err)
			assert.Equal(suite.T(), tc.status, w.Code)
		})
	}
}

func (suite *BinaryTestSuite) TestRoutesAndInvalidNonce() {
	s := &Server{}
	engine := gin.New()
	s.injectBinaryAPI(engine)
	routes := []string{}
	for _, route := range engine.Routes() {
		routes = append(routes, route.Path)
	}
	assert.ElementsMatch(suite.T(), []string{"/messageBin/:msgid", "/messageBinByNonce/:pid/:nonce", "/assignmentBinByNonce/:pid/:nonce", "/assignmentBinByMessage/:msgid"}, routes)
	for _, path := range []string{"/messageBinByNonce/pid/no", "/assignmentBinByNonce/pid/-1"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	}
}

func TestBinaryTestSuite(t *testing.T) { suite.Run(t, new(BinaryTestSuite)) }
