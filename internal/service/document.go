package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"
	"unicode/utf8"

	"rag-engine-go/internal/model"
	"rag-engine-go/pkg/docparser"

	"github.com/google/uuid"
)

// DocumentService — 文档上传 → 解析 → 分块 → (可选)Embedding → 索引 全流程编排
// 支持两种模式:
//   - Full mode: Tika + Ollama + ES (docker compose 启动)
//   - Standalone mode: 纯 Go 本地搜索引擎 (零依赖，开箱即用)
type DocumentService struct {
	parser   *ParserService
	chunker  *ChunkerService
	embedder *EmbedderService
	indexer  *IndexerService
	searcher *SearcherService
	qa       *QAService
	local    *LocalSearch // 本地检索引擎 (standalone fallback)
	docs     map[string]*model.Document
	useLocal bool // true → 使用本地搜索引擎
}

func NewDocumentService(
	parser *ParserService,
	chunker *ChunkerService,
	embedder *EmbedderService,
	indexer *IndexerService,
	searcher *SearcherService,
	qa *QAService,
) *DocumentService {
	ds := &DocumentService{
		parser:   parser,
		chunker:  chunker,
		embedder: embedder,
		indexer:  indexer,
		searcher: searcher,
		qa:       qa,
		local:    NewLocalSearch(),
		docs:     make(map[string]*model.Document),
	}

	// ES 或 Ollama 不可用 → 自动切换到本地搜索引擎
	if indexer == nil || searcher == nil || !embedder.Health() || !indexer.IndexExists() {
		ds.useLocal = true
	}

	return ds
}

// ProcessDocument ★ 上传 → 解析 → 分块 → 索引
// 在 standalone 模式下使用本地搜索引擎
func (s *DocumentService) ProcessDocument(file multipart.File, header *multipart.FileHeader) (*model.Document, error) {
	docID := uuid.New().String()

	// 1. 读取原始文件数据
	rawData, _ := io.ReadAll(file)

	// 2. 本地解析器优先（纯Go PDF/DOCX/TXT）
	content, err := docparser.Parse(rawData, header.Filename)
	if err != nil {
		// 本地解析失败，尝试原始文本兜底
		if isPlainText(rawData) {
			content = string(rawData)
		} else {
			return nil, fmt.Errorf("无法解析该文档格式: %v", err)
		}
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("文档内容为空")
	}

	// 2. 文本分块
	chunks, err := s.chunker.Split(content)
	if err != nil {
		return nil, fmt.Errorf("分块失败: %w", err)
	}

	// 3. 索引
	doc := &model.Document{
		ID: docID, Filename: header.Filename, Size: header.Size,
		MimeType: header.Header.Get("Content-Type"),
		Content: content, ChunkCount: len(chunks), CreatedAt: time.Now(),
	}

	if s.useLocal {
		// ★ 本地检索引擎
		for i, c := range chunks {
			s.local.IndexChunk(fmt.Sprintf("%s-%d", docID, i), docID, header.Filename, c.Text, i)
		}
	} else {
		// ES 模式: 生成 Embedding + 索引
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
		}
		embeddings, err := s.embedder.EmbedBatch(texts)
		if err != nil {
			return nil, fmt.Errorf("Embedding 失败: %w (可尝试 ollama pull %s)", err, s.embedder.model)
		}
		for i, c := range chunks {
			esChunk := model.Chunk{
				ChunkID: fmt.Sprintf("%s-%d", docID, i),
				DocID: docID, DocName: header.Filename, Index: i,
				Text: c.Text, Embedding: embeddings[i], CreatedAt: time.Now(),
			}
			s.indexer.IndexChunk(esChunk)
		}
	}

	s.docs[docID] = doc
	return doc, nil
}

// Search 检索
func (s *DocumentService) Search(query string, topK int, docID string) (*model.SearchResponse, error) {
	if s.useLocal {
		return s.local.Search(query, topK, docID), nil
	}

	queryEmbedding, err := s.embedder.EmbedSingle(query)
	if err != nil {
		return s.searcher.KeywordSearch(query, topK)
	}
	return s.searcher.Search(query, queryEmbedding, topK, docID)
}

// Ask 问答
func (s *DocumentService) Ask(question string, topK int, docID string) (string, []model.SearchResult, error) {
	results, err := s.Search(question, topK, docID)
	if err != nil {
		return "", nil, err
	}
	if s.qa.IsEnabled() {
		answer, err := s.qa.AskSync(question, results.Results)
		if err != nil {
			return SearchOnlyAnswer(results.Results), results.Results, nil
		}
		return answer, results.Results, nil
	}
	return SearchOnlyAnswer(results.Results), results.Results, nil
}

// AskStream 流式问答
func (s *DocumentService) AskStream(question string, topK int, docID string, onToken func(string)) error {
	results, err := s.Search(question, topK, docID)
	if err != nil {
		onToken(fmt.Sprintf("搜索错误: %v", err))
		onToken("[DONE]")
		return err
	}
	if !s.qa.IsEnabled() {
		onToken(SearchOnlyAnswer(results.Results))
		onToken("[DONE]")
		return nil
	}
	return s.qa.AskStream(question, results.Results, onToken)
}

func (s *DocumentService) ListDocs() []*model.Document {
	list := make([]*model.Document, 0, len(s.docs))
	for _, d := range s.docs {
		list = append(list, d)
	}
	return list
}

func (s *DocumentService) QAEnabled() bool { return s.qa.IsEnabled() }

func (s *DocumentService) Stats() map[string]interface{} {
	count := int64(len(s.docs))
	if s.useLocal {
		count = s.local.DocCount()
	}
	return map[string]interface{}{
		"total_docs":  len(s.docs),
		"chunks":      count,
		"mode":        s.Mode(),
		"qa_enabled":  s.qa.IsEnabled(),
	}
}

// DeleteDoc removes a document from both the in-memory store and search index
func (s *DocumentService) DeleteDoc(docID string) error {
	if _, ok := s.docs[docID]; !ok {
		return fmt.Errorf("document not found: %s", docID)
	}
	delete(s.docs, docID)
	if s.useLocal {
		s.local.DeleteByDocID(docID)
	} else if s.indexer != nil {
		s.indexer.DeleteByDocID(docID)
	}
	return nil
}

func (s *DocumentService) Mode() string {
	if s.useLocal {
		return "standalone (local search engine, zero dependencies)"
	}
	return "full (ES + Ollama + Tika)"
}

// isPlainText 检查是否为可读文本
func isPlainText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	checkLen := 4096
	if len(data) < checkLen {
		checkLen = len(data)
	}
	nonPrintable := 0
	for _, b := range data[:checkLen] {
		if b != 0 && b != '\n' && b != '\r' && b != '\t' && b < 0x20 && b >= 0x01 {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(checkLen) < 0.05
}

func init() { _ = strings.TrimSpace; _ = utf8.RuneCountInString }
