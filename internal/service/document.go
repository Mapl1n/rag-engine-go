package service

import (
	"fmt"
	"mime/multipart"
	"time"

	"rag-engine-go/internal/model"

	"github.com/google/uuid"
)

// DocumentService — 文档上传 → 解析 → 分块 → Embedding → 索引 全流程编排
type DocumentService struct {
	parser   *ParserService
	chunker  *ChunkerService
	embedder *EmbedderService
	indexer  *IndexerService
	searcher *SearcherService
	qa       *QAService
	docs     map[string]*model.Document // in-memory doc store (replace with DB in production)
}

func NewDocumentService(
	parser *ParserService,
	chunker *ChunkerService,
	embedder *EmbedderService,
	indexer *IndexerService,
	searcher *SearcherService,
	qa *QAService,
) *DocumentService {
	return &DocumentService{
		parser:   parser,
		chunker:  chunker,
		embedder: embedder,
		indexer:  indexer,
		searcher: searcher,
		qa:       qa,
		docs:     make(map[string]*model.Document),
	}
}

// ProcessDocument ★ 核心流程：上传 → 解析 → 分块 → Embedding → 索引
func (s *DocumentService) ProcessDocument(file multipart.File, header *multipart.FileHeader) (*model.Document, error) {
	docID := uuid.New().String()

	// 1. Tika 解析
	content, err := s.parser.Parse(file, header.Filename)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("文档内容为空或无法解析")
	}

	// 2. 文本分块
	chunks, err := s.chunker.Split(content)
	if err != nil {
		return nil, fmt.Errorf("chunk: %w", err)
	}

	// 3. 生成 Embedding
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}
	embeddings, err := s.embedder.EmbedBatch(texts)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	// 4. 索引到 ES
	doc := &model.Document{
		ID:         docID,
		Filename:   header.Filename,
		Size:       header.Size,
		MimeType:   header.Header.Get("Content-Type"),
		Content:    content,
		ChunkCount: len(chunks),
		CreatedAt:  time.Now(),
	}

	for i, c := range chunks {
		esChunk := model.Chunk{
			ChunkID:   fmt.Sprintf("%s-%d", docID, i),
			DocID:     docID,
			DocName:   header.Filename,
			Index:     i,
			Text:      c.Text,
			Embedding: embeddings[i],
			CreatedAt: time.Now(),
		}
		s.indexer.IndexChunk(esChunk)
	}

	s.docs[docID] = doc
	return doc, nil
}

// Search 语义检索
func (s *DocumentService) Search(query string, topK int, docID string) (*model.SearchResponse, error) {
	// 获取查询向量
	queryEmbedding, err := s.embedder.EmbedSingle(query)
	if err != nil {
		// 降级到纯 BM25
		return s.searcher.KeywordSearch(query, topK)
	}

	return s.searcher.Search(query, queryEmbedding, topK, docID)
}

// Ask 问答：检索 + (可选) LLM 回答
func (s *DocumentService) Ask(question string, topK int, docID string) (string, []model.SearchResult, error) {
	results, err := s.Search(question, topK, docID)
	if err != nil {
		return "", nil, err
	}

	// 有 LLM → 生成回答
	if s.qa.IsEnabled() {
		answer, err := s.qa.AskSync(question, results.Results)
		if err != nil {
			return SearchOnlyAnswer(results.Results), results.Results, nil
		}
		return answer, results.Results, nil
	}

	// 无 LLM → 返回检索结果摘要
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

// ListDocs 列出所有文档
func (s *DocumentService) ListDocs() []*model.Document {
	list := make([]*model.Document, 0, len(s.docs))
	for _, d := range s.docs {
		list = append(list, d)
	}
	return list
}

// QAEnabled 是否启用 LLM
func (s *DocumentService) QAEnabled() bool {
	return s.qa.IsEnabled()
}

// Stats 返回统计信息
func (s *DocumentService) Stats() map[string]interface{} {
	count, _ := s.searcher.DocCount()
	return map[string]interface{}{
		"total_docs":     len(s.docs),
		"qa_enabled":     s.qa.IsEnabled(),
		"indexed_chunks": count,
	}
}
