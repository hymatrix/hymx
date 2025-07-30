package server

import (
	"net/http"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	"github.com/hymatrix/hymx/node/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
)

var log = common.NewLog("server")

type Server struct {
	node *node.Node

	apiServer *http.Server
}

func New(
	bundler *goar.Bundler, redisURL, arweaveURL, hymxURL string, nodeInfo *schema.Info,
) *Server {
	return &Server{
		node: node.New(bundler, redisURL, arweaveURL, hymxURL, nodeInfo),
	}
}

func (s *Server) Run(endpoint string) {
	go s.runAPI(endpoint)

	s.node.Run()
}

func (s *Server) Close() {
	log.Info("server is shutting down")
	s.closeAPI()
	s.node.Close()
	log.Info("server has been shut down")
}

func (s *Server) Mount(moduleFormat string, spawner vmmSchema.VmSpawnFunc) error {
	return s.node.Mount(moduleFormat, spawner)
}

func (s *Server) AddResultHandler(handlers ...schema.ResultHandler) {
	s.node.AddResultHandler(handlers...)
}
