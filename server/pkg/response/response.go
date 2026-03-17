package response

import (
	"errors"
	"fmt"
	"net/http"

	"backupx/server/internal/apperror"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: "OK", Message: "success", Data: data})
}

func Error(c *gin.Context, err error) {
	fmt.Printf("HTTP Error: %v\n", err)
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.Status, Envelope{Code: appErr.Code, Message: appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, Envelope{Code: "INTERNAL_ERROR", Message: "服务器内部错误"})
}
