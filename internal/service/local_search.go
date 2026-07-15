package service

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"rag-engine-go/internal/model"
)

type LocalSearch struct {
	mu     sync.RWMutex
	chunks []localChunk
}

type localChunk struct {
	ChunkID   string
	DocID     string
	DocName   string
	Index     int
	Text      string
	CreatedAt time.Time
}

func NewLocalSearch() *LocalSearch { return &LocalSearch{} }

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

// Search 本地 BM25-like 全文搜索
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
			DocName: r.chunk.DocName, Text: bestWindow(r.chunk.Text, queryTerms, 300),
			Score: r.score, Index: r.chunk.Index,
		}
	}

	return &model.SearchResponse{
		Results: results, Total: len(results),
		Took: time.Since(start).Milliseconds(), Query: query,
	}
}

// scoreBM25 — 简化的 BM25 评分
func scoreBM25(text string, queryTerms []string) float64 {
	docLen := float64(utf8.RuneCountInString(text))
	k1, b, avgDL := 1.2, 0.75, 300.0
	score := 0.0
	textLower := strings.ToLower(text)

	for _, term := range queryTerms {
		// guard: term longer than text = no match
		if len(term) > len(text) {
			continue
		}
		tf := float64(countOccurrences(textLower, term))
		if tf == 0 {
			continue
		}
		numerator := tf * (k1 + 1)
		denominator := tf + k1*(1-b+b*(docLen/avgDL))
		score += numerator / denominator
	}
	return score
}

// countOccurrences 统计 term 在 text 中出现次数
func countOccurrences(text, term string) int {
	if len(term) > len(text) {
		return 0
	}
	n := 0
	for i := 0; i <= len(text)-len(term); i++ {
		if text[i:i+len(term)] == term {
			n++
		}
	}
	return n
}

// tokenize — 中文 bigram + 英文单词分词
// 不再对 CJK 逐字 tokenize，避免单字匹配过多噪声
func tokenize(text string) []string {
	seen := make(map[string]bool)
	var tokens []string

	// CJK bigrams
	runes := []rune(text)
	for i := 0; i < len(runes)-1; i++ {
		bigram := string(runes[i : i+2])
		if !seen[bigram] {
			seen[bigram] = true
			tokens = append(tokens, bigram)
		}
	}

	// 英文单词 (space/标点分割)
	for _, word := range strings.Fields(text) {
		w := strings.ToLower(strings.TrimFunc(word, func(r rune) bool {
			return r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':'
		}))
		if len(w) >= 2 && !seen[w] {
			seen[w] = true
			tokens = append(tokens, w)
		}
	}

	return tokens
}

// bestWindow — 从文本中找到含最多匹配词的最佳窗口（runes 操作，避免 byte 截断）
func bestWindow(text string, queryTerms []string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}

	bestStart, bestScore := 0, 0
	step := maxRunes / 3
	if step < 10 {
		step = 10
	}

	// Slide in rune-space but count matches in byte-space for accuracy
	for i := 0; i <= len(runes)-maxRunes; i += step {
		winBytes := string(runes[i:min(i+maxRunes, len(runes))])
		score := 0
		for _, t := range queryTerms {
			score += countOccurrences(strings.ToLower(winBytes), strings.ToLower(t))
		}
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}

	end := bestStart + maxRunes
	if end > len(runes) {
		end = len(runes)
	}
	result := string(runes[bestStart:end])
	if bestStart > 0 {
		result = "…" + result
	}
	if end < len(runes) {
		result += "…"
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CosineSimilarity — 保留给外部使用
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
