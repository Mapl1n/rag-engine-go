package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbedderService — 调用 Ollama API 生成文本向量
// 默认使用 BGE-M3 (multilingual, 1024-dim)
type EmbedderService struct {
	ollamaURL string
	model     string
	client    *http.Client
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Input  string `json:"input"`
}

type ollamaEmbedResp struct {
	Embedding []float32 `json:"embedding"`
}

func NewEmbedderService(ollamaURL, model string) *EmbedderService {
	return &EmbedderService{
		ollamaURL: ollamaURL,
		model:     model,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

// EmbedSingle 生成单个文本的向量
func (s *EmbedderService) EmbedSingle(text string) ([]float32, error) {
	body := ollamaEmbedReq{Model: s.model, Input: text}
	data, _ := json.Marshal(body)

	resp, err := s.client.Post(
		s.ollamaURL+"/api/embed",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama unreachable (start with: ollama pull %s && ollama serve): %w", s.model, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
	}

	var result ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}
	return result.Embedding, nil
}

// EmbedBatch 批量生成向量（顺序调用，生产环境可并发）
func (s *EmbedderService) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := s.EmbedSingle(text)
		if err != nil {
			return nil, fmt.Errorf("batch[%d]: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// Health 检查 Ollama 是否在线
func (s *EmbedderService) Health() bool {
	resp, err := s.client.Get(s.ollamaURL + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
