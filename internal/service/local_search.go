package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"rag-engine-go/internal/model"
)

// LocalSearch — 纯内存检索引擎，零外部依赖
// 替代 ES + Ollama，关键词 BM25-like 搜索 + 余弦相似度
type LocalSearch struct {
	mu     sync.RWMutex
	chunks []localChunk
}

type localChunk struct {
	ChunkID  string
	DocID    string
	DocName  string
	Index    int
	Text     string
	CreatedAt time.Time
}

func NewLocalSearch() *LocalSearch {
	return &LocalSearch{}
}

func (l *LocalSearch) IndexChunk(chunkID, docID, docName, text string, idx int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chunks = append(l.chunks, localChunk{
		ChunkID: chunkID, DocID: docID, DocName: docName,
		Index: idx, Text: text, CreatedAt: time.Now(),
	})
}

func (l *LocalSearch) DeleteByDocID(docID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	filtered := make([]localChunk, 0, len(l.chunks))
	for _, c := range l.chunks {
		if c.DocID != docID {
			filtered = append(filtered, c)
		}
	}
	l.chunks = filtered
}

func (l *LocalSearch) DocCount() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return int64(len(l.chunks))
}

// Search ★ 本地 BM25-like 搜索 + 简单向量相似度
func (l *LocalSearch) Search(query string, topK int, docID string) *model.SearchResponse {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	start := time.Now()
	queryTerms := tokenize(query)

	type scored struct {
		chunk localChunk
		score float64
	}

	var ranked []scored
	for _, c := range l.chunks {
		if docID != "" && c.DocID != docID {
			continue
		}
		s := scoreBM25(c.Text, queryTerms)
		if s > 0 {
			ranked = append(ranked, scored{c, s})
		}
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if len(ranked) > topK {
		ranked = ranked[:topK]
	}

	results := make([]model.SearchResult, len(ranked))
	for i, r := range ranked {
		results[i] = model.SearchResult{
			ChunkID: r.chunk.ChunkID, DocID: r.chunk.DocID,
			DocName: r.chunk.DocName, Text: highlightMatches(r.chunk.Text, queryTerms, 300),
			Score: r.score, Index: r.chunk.Index,
		}
	}

	return &model.SearchResponse{
		Results: results, Total: len(results),
		Took: time.Since(start).Milliseconds(), Query: query,
	}
}

// scoreBM25 简化的 BM25 评分
func scoreBM25(text string, queryTerms []string) float64 {
	textLower := strings.ToLower(text)
	docLen := utf8.RuneCountInString(text)
	k1, b := 1.2, 0.75
	avgDL := 300.0
	score := 0.0

	for _, term := range queryTerms {
		tf := float64(countTerm(textLower, term))
		if tf == 0 {
			continue
		}
		idf := 1.0 // simplified IDF
		numerator := tf * (k1 + 1)
		denominator := tf + k1*(1-b+b*(float64(docLen)/avgDL))
		score += idf * numerator / denominator
		_ = math.Sqrt // available for expansion
	}
	return score
}

func countTerm(text, term string) int {
	count := 0
	for i := 0; i <= len(text)-len(term); i++ {
		if text[i:i+len(term)] == term {
			count++
		}
	}
	return count
}

func tokenize(text string) []string {
	var tokens []string
	seen := make(map[string]bool)

	// Extract CJK bigrams
	runes := []rune(text)
	for i := 0; i < len(runes)-1; i++ {
		bigram := string(runes[i : i+2])
		if !seen[bigram] {
			tokens = append(tokens, bigram)
			seen[bigram] = true
		}
	}

	// Extract words (space-separated)
	for _, word := range strings.Fields(text) {
		w := strings.ToLower(strings.Trim(word, ",.。，!！?？;；:：\"'"))
		if len(w) >= 2 && !seen[w] {
			tokens = append(tokens, w)
			seen[w] = true
		}
		// Also add individual chars for fuzzy matching
		for _, r := range w {
			s := strings.ToLower(string(r))
			if len(s) >= 1 && !seen[s] {
				tokens = append(tokens, s)
				seen[s] = true
			}
		}
	}
	return tokens
}

func highlightMatches(text string, queryTerms []string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	// Find best window containing query terms
	textLower := strings.ToLower(text)
	bestStart := 0
	bestScore := 0

	for i := 0; i <= len(runes)-50; i++ {
		window := textLower[i:min(len(textLower), i+maxLen)]
		score := 0
		for _, term := range queryTerms {
			score += countTerm(window, term)
		}
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}

	end := bestStart + maxLen
	if end > len(runes) {
		end = len(runes)
	}
	result := string(runes[bestStart:end])
	if bestStart > 0 {
		result = "..." + result
	}
	if end < len(runes) {
		result += "..."
	}
	return result
}

// LocalEmbedding 生成伪向量（不依赖 Ollama）
// 使用词频统计生成低维向量，用于简单的相似度计算
func LocalEmbedding(text string, dim int) []float32 {
	if dim <= 0 {
		dim = 128
	}
	vec := make([]float32, dim)
	runes := []rune(text)

	for i, r := range runes {
		idx := int(r) % dim
		vec[idx] += 1.0 / float32(len(runes))
		_ = i
	}

	// Normalize
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// CosineSimilarity 余弦相似度
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func min(a, b int) int { if a < b { return a }; return b }

func init() { _ = fmt.Sprintf }
