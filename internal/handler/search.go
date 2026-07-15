package handler

import (
	"fmt"
	"net/http"
	"strings"

	"rag-engine-go/internal/model"
	"rag-engine-go/internal/service"
	"rag-engine-go/pkg/response"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	docService *service.DocumentService
}

func NewSearchHandler(docService *service.DocumentService) *SearchHandler {
	return &SearchHandler{docService: docService}
}

// Search 语义检索
func (h *SearchHandler) Search(c *gin.Context) {
	var req model.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	result, err := h.docService.Search(req.Query, req.TopK, req.DocID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

// QA 问答（同步）
func (h *SearchHandler) QA(c *gin.Context) {
	var req model.QARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	answer, results, err := h.docService.Ask(req.Question, req.TopK, req.DocID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{
		"question": req.Question,
		"answer":   answer,
		"results":  results,
	})
}

// QAStream 流式问答（SSE）
func (h *SearchHandler) QAStream(c *gin.Context) {
	var req model.QARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	h.docService.AskStream(req.Question, req.TopK, req.DocID, func(token string) {
		if token == "[DONE]" {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		} else {
			// Escape newlines and quotes for SSE
			safe := strings.ReplaceAll(token, "\n", "\\n")
			fmt.Fprintf(c.Writer, "data: %s\n\n", safe)
		}
		c.Writer.Flush()
	})
}
