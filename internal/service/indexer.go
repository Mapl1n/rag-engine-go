package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"rag-engine-go/internal/config"
	"rag-engine-go/internal/model"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// IndexerService — ElasticSearch 索引管理
// 存储 chunk 文本 + dense_vector + BM25 字段
type IndexerService struct {
	es    *elasticsearch.Client
	index string
}

func NewIndexerService(cfg *config.Config) (*IndexerService, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{cfg.ESURL()},
	})
	if err != nil {
		return nil, fmt.Errorf("es client: %w", err)
	}

	// Ping
	resp, err := es.Ping()
	if err != nil {
		log.Printf("[ES] warning: cannot reach ElasticSearch at %s — will retry on index", cfg.ESURL())
	} else {
		resp.Body.Close()
		log.Printf("[ES] connected to %s", cfg.ESURL())
	}

	return &IndexerService{es: es, index: cfg.ESIndex}, nil
}

// CreateIndex 创建 ES 索引（含 dense_vector 映射）
func (s *IndexerService) CreateIndex() error {
	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"chunk_id":   map[string]string{"type": "keyword"},
				"doc_id":     map[string]string{"type": "keyword"},
				"doc_name":   map[string]string{"type": "keyword"},
				"index":      map[string]string{"type": "integer"},
				"text":       map[string]string{"type": "text", "analyzer": "standard"},
				"embedding": map[string]interface{}{
					"type":       "dense_vector",
					"dims":       1024,
					"index":      true,
					"similarity": "cosine",
				},
				"created_at": map[string]string{"type": "date"},
			},
		},
	}

	body, _ := json.Marshal(mapping)
	req := esapi.IndicesCreateRequest{
		Index: s.index,
		Body:  bytes.NewReader(body),
	}

	resp, err := req.Do(context.Background(), s.es)
	if err != nil {
		return fmt.Errorf("es create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		var e map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("es index error: %v", e)
	}
	log.Printf("[ES] index '%s' created with dense_vector mapping", s.index)
	return nil
}

// IndexChunk 索引单个 chunk 到 ES
func (s *IndexerService) IndexChunk(chunk model.Chunk) error {
	doc := map[string]interface{}{
		"chunk_id":   chunk.ChunkID,
		"doc_id":     chunk.DocID,
		"doc_name":   chunk.DocName,
		"index":      chunk.Index,
		"text":       chunk.Text,
		"embedding":  chunk.Embedding,
		"created_at": chunk.CreatedAt.Format(time.RFC3339),
	}

	body, _ := json.Marshal(doc)
	req := esapi.IndexRequest{
		Index:      s.index,
		DocumentID: chunk.ChunkID,
		Body:       bytes.NewReader(body),
		Refresh:    "true",
	}

	resp, err := req.Do(context.Background(), s.es)
	if err != nil {
		return fmt.Errorf("es index: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// IndexChunks 批量索引
func (s *IndexerService) IndexChunks(chunks []model.Chunk) error {
	for _, chunk := range chunks {
		if err := s.IndexChunk(chunk); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByDocID 删除某文档的所有 chunk
func (s *IndexerService) DeleteByDocID(docID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]string{"doc_id": docID},
		},
	}
	body, _ := json.Marshal(query)
	req := esapi.DeleteByQueryRequest{
		Index: []string{s.index},
		Body:  bytes.NewReader(body),
	}
	resp, err := req.Do(context.Background(), s.es)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ES returns the underlying elasticsearch client
func (s *IndexerService) ES() *elasticsearch.Client {
	return s.es
}

// IndexExists 检查索引是否存在
func (s *IndexerService) IndexExists() bool {
	req := esapi.IndicesExistsRequest{Index: []string{s.index}}
	resp, err := req.Do(context.Background(), s.es)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
