package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/server/schema"
)

func (s *Server) runAdminAPI(endpoint string) {
	engine := gin.Default()
	engine.Use(common.CORSMiddleware())
	engine.POST("/admin/vms/stop", s.Stop)
	engine.POST("/admin/vms/resume", s.Resume)
	engine.GET("/admin/vms/running", s.Running)

	s.adminAPIServer = &http.Server{
		Addr:    endpoint,
		Handler: engine,
	}

	if err := s.adminAPIServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("admin http ListenAndServe", "err", err)
	}
}

func (s *Server) closeAdminAPI() {
	log.Info("admin api is shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := s.adminAPIServer.Shutdown(ctx); err != nil {
		log.Error("failed to shut down the admin api", "err", err)
		s.adminAPIServer.Close()
		return
	}
	log.Info("admin api has been shut down")
}

func (s *Server) Stop(c *gin.Context) {
	req := schema.RequestVM{}
	if err := c.ShouldBindJSON(&req); err != nil || req.Pid == "" {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	if err := s.node.Stop(req.Pid); err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, schema.Response{Id: req.Pid, Message: "stopped"})
}

func (s *Server) Resume(c *gin.Context) {
	req := schema.RequestVM{}
	if err := c.ShouldBindJSON(&req); err != nil || req.Pid == "" {
		schema.ErrorResponse(c, schema.ErrInvalidParams.Error())
		return
	}
	if err := s.node.Resume(req.Pid); err != nil {
		schema.ErrorResponse(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, schema.Response{Id: req.Pid, Message: "resumed"})
}

func (s *Server) Running(c *gin.Context) {
	c.JSON(http.StatusOK, s.node.Running())
}
