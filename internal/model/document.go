package model

import "time"

// Document 原始文档
type Document struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	MimeType   string    `json:"mime_type"`
	Content    string    `json:"content"`       // 解析后的纯文本
	MinioPath  string    `json:"minio_path"`
	ChunkCount int       `json:"chunk_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// Chunk 文本分块（存入 ES）
type Chunk struct {
	ChunkID    string    `json:"chunk_id"`
	DocID      string    `json:"doc_id"`
	DocName    string    `json:"doc_name"`
	Index      int       `json:"index"`
	Text       string    `json:"text"`
	Embedding  []float32 `json:"embedding,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SearchRequest 检索请求
type SearchRequest struct {
	Query    string `json:"query" binding:"required"`
	TopK     int    `json:"top_k"`
	DocID    string `json:"doc_id,omitempty"` // 可选：限定文档范围
}

// SearchResult 检索结果
type SearchResult struct {
	ChunkID  string  `json:"chunk_id"`
	DocID    string  `json:"doc_id"`
	DocName  string  `json:"doc_name"`
	Text     string  `json:"text"`
	Score    float64 `json:"score"`
	Index    int     `json:"index"`
	Highlights []string `json:"highlights,omitempty"`
}

// SearchResponse 检索响应
type SearchResponse struct {
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Took     int64          `json:"took_ms"`
	Query    string         `json:"query"`
}

// QARequest 问答请求
type QARequest struct {
	Question string `json:"question" binding:"required"`
	DocID    string `json:"doc_id,omitempty"`
	TopK     int    `json:"top_k"`
}

// QAStreamChunk SSE 流式问答块
type QAStreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}
