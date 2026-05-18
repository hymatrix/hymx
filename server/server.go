package server

import (
	"net/http"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/node"
	"github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/pay"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

var log = common.NewLog("server")

type vmAdmin interface {
	Stop(pid string) error
	Resume(pid string) error
	Running() []string
}

type Server struct {
	node *node.Node
	pay  *pay.Pay

	vmAdmin vmAdmin

	apiServer      *http.Server
	adminAPIServer *http.Server
}

func New(node *node.Node, pay *pay.Pay) *Server {
	return &Server{
		node:    node,
		pay:     pay,
		vmAdmin: node,
	}
}

func (s *Server) Run(endpoint, adminEndpoint, startMode string) {
	if s.pay != nil {
		s.pay.LoadCheckpoint()
		s.pay.Run()
		s.AddResultHandler(s.pay.HymxDepositHandler)
		s.AddItemHandler(s.pay.HymxFeeHandler)
	}

	go s.runAPI(endpoint)
	if adminEndpoint != "" {
		go s.runAdminAPI(adminEndpoint)
	}

	s.node.Run(startMode)
}

func (s *Server) Close() {
	log.Info("server is shutting down")
	s.closeAPI()
	s.closeAdminAPI()
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

func (s *Server) AddAssignResHandler(handlers ...schema.AssignResHandler) {
	s.node.AddAssignResHandler(handlers...)
}
