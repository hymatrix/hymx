package schema

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RespErr struct {
	Err string `json:"error"`
}

func (r RespErr) Error() string {
	return r.Err
}

func ErrorResponse(c *gin.Context, err string) {
	// client error
	c.JSON(http.StatusBadRequest, RespErr{
		Err: err,
	})
}
