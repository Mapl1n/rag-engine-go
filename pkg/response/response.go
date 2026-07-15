package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type R struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, R{0, "ok", data})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, R{code, msg, nil})
}

func Stream(c *gin.Context) *gin.Context {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
	return c
}
