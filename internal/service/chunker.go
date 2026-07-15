package service

import (
	"unicode/utf8"
)

// ChunkerService — 文本智能分块
type ChunkerService struct {
	chunkSize    int
	chunkOverlap int
}

func NewChunkerService(chunkSize, chunkOverlap int) *ChunkerService {
	return &ChunkerService{chunkSize: chunkSize, chunkOverlap: chunkOverlap}
}

type ChunkResult struct {
	Text  string
	Index int
}

// Split 将长文本切分为有重叠的 chunk
func (s *ChunkerService) Split(text string) ([]ChunkResult, error) {
	if utf8.RuneCountInString(text) <= s.chunkSize {
		return []ChunkResult{{Text: text, Index: 0}}, nil
	}

	var chunks []ChunkResult
	runes := []rune(text)
	step := s.chunkSize - s.chunkOverlap
	if step <= 0 {
		step = s.chunkSize
	}

	for i := 0; i < len(runes); i += step {
		end := i + s.chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunkText := string(runes[i:end])
		chunks = append(chunks, ChunkResult{Text: chunkText, Index: len(chunks)})
		if end >= len(runes) {
			break
		}
	}

	return chunks, nil
}

// splitSentences — 按中英文标点分句
func splitSentences(text string) []string {
	var result []string
	var cur []rune

	for _, r := range []rune(text) {
		cur = append(cur, r)
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n' {
			if len(cur) > 1 {
				result = append(result, string(cur))
			}
			cur = nil
		}
	}
	if len(cur) > 0 {
		result = append(result, string(cur))
	}
	return result
}

// GetOverlapContext 获取 chunk 前后的 overlap 文本
func (s *ChunkerService) GetOverlapContext(chunks []ChunkResult, idx int) string {
	if s.chunkOverlap <= 0 || len(chunks) == 0 {
		return chunks[idx].Text
	}
	if len(chunks) == 1 {
		return chunks[0].Text
	}

	var result string

	// 前一个 chunk 的尾部
	if idx > 0 {
		prev := chunks[idx-1].Text
		if utf8.RuneCountInString(prev) > s.chunkOverlap {
			prevRunes := []rune(prev)
			result += string(prevRunes[len(prevRunes)-s.chunkOverlap:]) + "\n---\n"
		}
	}

	// 当前 chunk
	result += chunks[idx].Text

	// 后一个 chunk 的头部
	if idx < len(chunks)-1 {
		next := chunks[idx+1].Text
		result += "\n---\n"
		if utf8.RuneCountInString(next) > s.chunkOverlap {
			nextRunes := []rune(next)
			result += string(nextRunes[:s.chunkOverlap])
		}
	}

	return result
}
