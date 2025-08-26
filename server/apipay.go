package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) injectPayApi(engine *gin.Engine) {
	if s.pay == nil {
		return
	}

	engine.GET("/pay/info", s.PayInfo)
	engine.GET("pay/sponsorTotal/:sponsor", s.PaySponsorTotal)
	engine.GET("/pay/sponsorBreakdown/:sponsor", s.PaySponsorBreakdown)
	engine.GET("/pay/beneficiaryTotal/:beneficiary", s.PayBeneficiaryTotal)
	engine.GET("/pay/beneficiaryBreakdown/:beneficiary", s.PayBeneficiaryBreakdown)
	engine.GET("/pay/totalPending/:beneficiary", s.PayTotalPending)
}

func (s *Server) PayInfo(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.Info())
}

func (s *Server) PaySponsorTotal(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.SponsorTotal(c.Param("sponsor")))
}

func (s *Server) PaySponsorBreakdown(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.SponsorBreakdown(c.Param("sponsor")))
}

func (s *Server) PayBeneficiaryTotal(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.BeneficiaryTotal(c.Param("beneficiary")))
}

func (s *Server) PayBeneficiaryBreakdown(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.BeneficiaryBreakdown(c.Param("beneficiary")))
}

func (s *Server) PayTotalPending(c *gin.Context) {
	c.JSON(http.StatusOK, s.pay.TotalPending(c.Param("beneficiary")))
}
