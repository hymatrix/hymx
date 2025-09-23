package server

import (
	"net/http"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/pay"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
)

var log = common.NewLog("server")

type Server struct {
	node *node.Node
	pay  *pay.Pay

	apiServer *http.Server
}

func New(
	bundler *goar.Bundler, redisURL, arweaveURL, hymxURL string,
	nodeInfo *schema.Info, pay *pay.Pay,
) *Server {
	return &Server{
		node: node.New(bundler, redisURL, arweaveURL, hymxURL, nodeInfo),
		pay:  pay,
	}
}

func (s *Server) Run(endpoint string) {
	if s.pay != nil {
		s.pay.LoadCheckpoint()
		s.pay.Run()
		s.AddResultHandler(s.pay.HymxDepositHandler)
		s.AddItemHandler(s.pay.HymxFeeHandler)
	}

	go s.runAPI(endpoint)

	s.node.Run()
}

func (s *Server) Close() {
	log.Info("server is shutting down")
	s.closeAPI()
	s.node.Close()

	// close payment middleware
	if s.pay != nil {
		s.pay.Close()
		s.pay.SaveCheckpoint()
	}
	log.Info("server has been shut down")
}

func (s *Server) Mount(moduleFormat string, spawner vmmSchema.VmSpawnFunc) error {
	return s.node.Mount(moduleFormat, spawner)
}

func (s *Server) AddItemHandler(handlers ...schema.ItemHandler) {
	s.node.AddItemHandler(handlers...)
}

func (s *Server) AddResultHandler(handlers ...schema.ResultHandler) {
	s.node.AddResultHandler(handlers...)
}
