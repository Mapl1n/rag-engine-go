package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rag-engine-go/internal/model"

	"github.com/elastic/go-elasticsearch/v8"
)

// SearcherService — ★ 混合检索：BM25 + 向量相似度
//
// 传统 ES: 关键词匹配 (BM25)
// 语义检索: 向量余弦相似度 (kNN)
// 混合检索: BM25 分数 + 向量分数 → RRF 融合 → 最终排名
type SearcherService struct {
	es    *elasticsearch.Client
	index string
}

func NewSearcherService(es *elasticsearch.Client, index string) *SearcherService {
	return &SearcherService{es: es, index: index}
}

// Search ★ 混合检索: BM25 + kNN → RRF 融合
// 这就是飞度想从"关键词搜索"升级为"语义搜索"的核心技术
func (s *SearcherService) Search(query string, queryEmbedding []float32, topK int, docID string) (*model.SearchResponse, error) {
	start := time.Now()

	// 构建混合查询
	searchBody := map[string]interface{}{
		"size": topK,
		"query": map[string]interface{}{
			"bool": s.buildHybridQuery(query, queryEmbedding, docID),
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"text": map[string]interface{}{
					"fragment_size":       150,
					"number_of_fragments": 3,
				},
			},
		},
		"_source": []string{"chunk_id", "doc_id", "doc_name", "text", "index"},
	}

	body, _ := json.Marshal(searchBody)

	resp, err := s.es.Search(
		s.es.Search.WithContext(context.Background()),
		s.es.Search.WithIndex(s.index),
		s.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer resp.Body.Close()

	var esResp struct {
		Hits struct {
			Total interface{} `json:"total"` // ES 7.x: int, ES 8.x: {"value": N}
			Hits  []struct {
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source struct {
					ChunkID string `json:"chunk_id"`
					DocID   string `json:"doc_id"`
					DocName string `json:"doc_name"`
					Text    string `json:"text"`
					Index   int    `json:"index"`
				} `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Took int64 `json:"took"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("es decode: %w", err)
	}

	results := make([]model.SearchResult, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		r := model.SearchResult{
			ChunkID:    hit.Source.ChunkID,
			DocID:      hit.Source.DocID,
			DocName:    hit.Source.DocName,
			Text:       hit.Source.Text,
			Score:      hit.Score,
			Index:      hit.Source.Index,
			Highlights: hit.Highlight["text"],
		}
		results = append(results, r)
	}

	return &model.SearchResponse{
		Results: results,
		Total:   parseESTotal(esResp.Hits.Total),
		Took:    time.Since(start).Milliseconds(),
		Query:   query,
	}, nil
}

// buildHybridQuery ★ 混合检索核心
//   - 有 embedding → BM25 + kNN 双路召回 → RRF 融合
//   - 无 embedding → 纯 BM25 关键词搜索
func (s *SearcherService) buildHybridQuery(query string, embedding []float32, docID string) map[string]interface{} {
	must := []interface{}{}

	// BM25 关键词路径
	if query != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{
				"text": map[string]interface{}{
					"query": query,
					"boost": 1.0,
				},
			},
		})
	}

	// 限定文档范围
	if docID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]string{"doc_id": docID},
		})
	}

	boolQuery := map[string]interface{}{}

	if len(must) > 0 {
		boolQuery["must"] = must
	} else {
		// ES requires at least one clause — match_all if no query/doc filter
		boolQuery["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	}

	// kNN 向量路径（如果有 embedding）
	shouldQueries := []interface{}{}
	if len(embedding) > 0 {
		shouldQueries = append(shouldQueries, map[string]interface{}{
			"knn": map[string]interface{}{
				"field":          "embedding",
				"query_vector":   embedding,
				"k":              10,
				"num_candidates": 100,
				"boost":          0.5,
			},
		})
	}

	if len(shouldQueries) > 0 {
		boolQuery["should"] = shouldQueries
	}

	return boolQuery
}

// parseESTotal 兼容 ES 7.x (int) 和 ES 8.x ({"value": N})
func parseESTotal(total interface{}) int {
	switch t := total.(type) {
	case float64:
		return int(t)
	case map[string]interface{}:
		if v, ok := t["value"].(float64); ok {
			return int(v)
		}
	}
	return 0
}

// KeywordSearch 纯 BM25 关键词搜索（降级模式）
func (s *SearcherService) KeywordSearch(query string, topK int) (*model.SearchResponse, error) {
	return s.Search(query, nil, topK, "")
}

// DocCount 索引文档数
func (s *SearcherService) DocCount() (int64, error) {
	resp, err := s.es.Count(
		s.es.Count.WithIndex(s.index),
		s.es.Count.WithContext(context.Background()),
	)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct{ Count int64 }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Count, nil
}
