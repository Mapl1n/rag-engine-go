package handler

import (
	"net/http"

	"rag-engine-go/internal/service"
	"rag-engine-go/pkg/response"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	docService *service.DocumentService
}

func NewUploadHandler(docService *service.DocumentService) *UploadHandler {
	return &UploadHandler{docService: docService}
}

// Upload 上传文档 → 自动解析/分块/Embedding/索引
func (h *UploadHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	doc, err := h.docService.ProcessDocument(file, header)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":          doc.ID,
		"filename":    doc.Filename,
		"size":        doc.Size,
		"chunk_count": doc.ChunkCount,
		"content_len": len(doc.Content),
		"message":     "文档处理完成",
	})
}

// ListDocs 列出已上传的文档
func (h *UploadHandler) ListDocs(c *gin.Context) {
	docs := h.docService.ListDocs()
	type docInfo struct {
		ID         string `json:"id"`
		Filename   string `json:"filename"`
		Size       int64  `json:"size"`
		ChunkCount int    `json:"chunk_count"`
		CreatedAt  string `json:"created_at"`
	}
	var list []docInfo
	for _, d := range docs {
		list = append(list, docInfo{
			ID:         d.ID,
			Filename:   d.Filename,
			Size:       d.Size,
			ChunkCount: d.ChunkCount,
			CreatedAt:  d.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	response.OK(c, list)
}

// Stats 系统统计
func (h *UploadHandler) Stats(c *gin.Context) {
	response.OK(c, h.docService.Stats())
}
