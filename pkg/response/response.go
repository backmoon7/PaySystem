package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess       = 0
	CodeParamError    = 400
	CodeUnauthorized  = 401
	CodeForbidden     = 403
	CodeNotFound      = 404
	CodeServerError   = 500
	CodeBusinessError = 1000
)

const (
	CodeOrderNotFound      = 1001
	CodeOrderStatusInvalid = 1002
	CodeBalanceNotEnough   = 1003
	CodeDuplicateRequest   = 1004
	CodeAccountNotFound    = 1005
	CodePaymentFailed      = 1006
	CodeRefundFailed       = 1007
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

func ParamError(c *gin.Context, message string) {
	Error(c, CodeParamError, message)
}

func ServerError(c *gin.Context, message string) {
	Error(c, CodeServerError, message)
}

func BusinessError(c *gin.Context, code int, message string) {
	Error(c, code, message)
}
