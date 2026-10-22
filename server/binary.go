package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/server/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (s *Server) GetMessageBin(c *gin.Context) {
	item, err := s.node.GetMessage(c.Param("msgid"))
	writeItemBinary(c, item, err)
}

func (s *Server) GetAssignBinByMsg(c *gin.Context) {
	item, err := s.node.GetAssignByMessage(c.Param("msgid"))
	writeItemBinary(c, item, err)
}

func (s *Server) GetMessageBinByNonce(c *gin.Context) {
	nonce, err := strconv.ParseInt(c.Param("nonce"), 10, 64)
	if err != nil || nonce < 0 {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	item, err := s.node.GetMessageByNonce(c.Param("pid"), nonce)
	writeItemBinary(c, item, err)
}

func (s *Server) GetAssignBinByNonce(c *gin.Context) {
	nonce, err := strconv.ParseInt(c.Param("nonce"), 10, 64)
	if err != nil || nonce < 0 {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	item, err := s.node.GetAssignByNonce(c.Param("pid"), nonce)
	writeItemBinary(c, item, err)
}

func writeItemBinary(c *gin.Context, item *goarSchema.BundleItem, err error) {
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, nil)
		return
	}
	// Redis stores JSON without Binary; preserve the signed TagsBy when rebuilding.
	binary, err := goarUtils.GenerateItemBinary(*item)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", binary)
}
