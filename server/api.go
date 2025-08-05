package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/server/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (s *Server) runAPI(endpoint string) {
	engine := gin.Default()
	// middleware
	engine.Use(common.CORSMiddleware())

	engine.GET("/info", s.Info)
	engine.GET("/callback", s.Callback)

	// api post message
	engine.POST("/", s.Submit)
	engine.GET("/result/:msgid", s.GetResult)
	engine.GET("/results/:pid", s.GetResults)

	// api for get message and assignment by nonce
	engine.GET("/message/:msgid", s.GetMessage)
	engine.GET("/messageByNonce/:pid/:nonce", s.GetMessageByNonce)
	engine.GET("/assignmentByNonce/:pid/:nonce", s.GetAssignByNonce)
	engine.GET("/assignmentByMessage/:msgid", s.GetAssignByMsg)

	// api for node registry
	engine.GET("/nodes", s.GetNodes)
	engine.GET("/node/:accid", s.GetNode)
	engine.GET("/nodesByProcess/:pid", s.GetNodesByProcess)
	engine.GET("/processes/:accid", s.GetProcesses)

	// api for core token
	engine.GET("/balanceof/:accid", s.BalanceOf)
	engine.GET("/stakeof/:accid", s.StakeOf)

	// cache for status query
	engine.GET("/cache/:pid/:key", s.GetCache)

	s.apiServer = &http.Server{
		Addr:    endpoint,
		Handler: engine,
	}

	if err := s.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("http ListenAndServe", "err", err)
	}
}

func (s *Server) closeAPI() {
	log.Info("api is shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := s.apiServer.Shutdown(ctx); err != nil {
		log.Error("failed to shut down the api", "err", err)
		s.apiServer.Close() // closed force
		return
	}
	log.Info("api has been shut down")
}

func (s *Server) Info(c *gin.Context) {
	c.JSON(http.StatusOK, s.node.Info())
}

func (s *Server) Callback(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		schema.ErrorResponse(c, schema.ErrCallbackFailed.Error())
		return
	}
	defer resp.Body.Close()

	c.JSON(http.StatusOK, schema.Response{
		Message: "ok",
	})
}

func (s *Server) Submit(c *gin.Context) {
	itemBinary, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error("read body failed", "err", err)
		schema.ErrorResponse(c, err.Error())
		return
	}
	item, err := goarUtils.DecodeBundleItem(itemBinary)
	if err != nil {
		log.Error("decode bundle item failed", "err", err)
		schema.ErrorResponse(c, err.Error())
		return
	}

	err = s.node.Handle(item)
	if err != nil {
		log.Error("handle item failed", "err", err)
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, schema.Response{
		Id: item.Id,
	})
}

func (s *Server) GetResult(c *gin.Context) {
	msgid := c.Param("msgid")
	// pid := c.Query("process-id")
	dbResult, err := s.node.GetResult(msgid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	if dbResult == nil {
		c.JSON(http.StatusOK, nil)
		return
	}

	c.JSON(http.StatusOK, dbResult)
}

func (s *Server) GetResults(c *gin.Context) {
	// GET /results/:pid?sort=DESC&limit=5

	pid := c.Param("pid")
	limit, err := strconv.ParseInt(c.Query("limit"), 10, 64)
	if err != nil {
		limit = 5
	}
	var results schema.ResponseResults
	dbResults, err := s.node.GetResults(pid, limit)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	for _, dbResult := range dbResults {

		timestamp, err := strconv.ParseInt(dbResult.Timestamp, 10, 64)
		if err != nil {
			schema.ErrorResponse(c, err.Error())
			return
		}
		nonce, err := strconv.ParseInt(dbResult.Nonce, 10, 64)
		if err != nil {
			schema.ErrorResponse(c, err.Error())
			return
		}
		cursor := schema.Cursor{
			Timestamp: timestamp,
			Ordinate:  nonce,
			Cron:      "1-10-minutes",
			Sort:      "ASC",
		}
		cursorBytes, err := json.Marshal(cursor)
		if err != nil {
			schema.ErrorResponse(c, err.Error())
			return
		}
		results.Edges = append(results.Edges, schema.ResultsEdge{
			Cursor: goarUtils.Base64Encode(cursorBytes),
			Node:   dbResult,
		})
	}

	c.JSON(http.StatusOK, results)
}

func (s *Server) GetMessage(c *gin.Context) {
	msgid := c.Param("msgid")
	res, err := s.node.GetMessage(msgid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetAssignByMsg(c *gin.Context) {
	msgid := c.Param("msgid")
	res, err := s.node.GetAssignByMessage(msgid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetMessageByNonce(c *gin.Context) {
	nonce, err := strconv.ParseInt(c.Param("nonce"), 10, 64)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	pid := c.Param("pid")

	res, err := s.node.GetMessageByNonce(pid, nonce)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	if res == nil {
		c.JSON(http.StatusNotFound, nil)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetAssignByNonce(c *gin.Context) {
	nonce, err := strconv.ParseInt(c.Param("nonce"), 10, 64)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	pid := c.Param("pid")

	res, err := s.node.GetAssignByNonce(pid, nonce)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	if res == nil {
		c.JSON(http.StatusNotFound, nil)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetNode(c *gin.Context) {
	accid := c.Param("accid")
	res, err := s.node.GetNode(accid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetNodes(c *gin.Context) {
	res, err := s.node.GetNodes()
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetProcesses(c *gin.Context) {
	accid := c.Param("accid")
	res, err := s.node.GetProcesses(accid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetNodesByProcess(c *gin.Context) {
	pid := c.Param("pid")
	res, err := s.node.GetNodesByProcess(pid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) BalanceOf(c *gin.Context) {
	accid := c.Param("accid")
	res, err := s.node.BalanceOf(accid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) StakeOf(c *gin.Context) {
	accid := c.Param("accid")
	res, err := s.node.StakeOf(accid)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) GetCache(c *gin.Context) {
	pid := c.Param("pid")
	key := c.Param("key")
	res, err := s.node.GetCache(pid, key)
	if err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}
