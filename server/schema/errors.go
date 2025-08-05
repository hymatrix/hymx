package schema

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	ErrInvalidParams  = errors.New("err_invalid_params")
	ErrCallbackFailed = errors.New("err_callback_failed")
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
