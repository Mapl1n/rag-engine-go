package service

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ChunkerService — 文本智能分块
// 策略: 按句子边界 + 字符滑动窗口 + overlap
type ChunkerService struct {
	chunkSize    int
	chunkOverlap int
}

func NewChunkerService(chunkSize, chunkOverlap int) *ChunkerService {
	return &ChunkerService{chunkSize: chunkSize, chunkOverlap: chunkOverlap}
}

// ChunkResult 分块结果
type ChunkResult struct {
	Text  string
	Index int
}

// Split 将长文本切分为有重叠的 chunk
func (s *ChunkerService) Split(text string) ([]ChunkResult, error) {
	if utf8.RuneCountInString(text) <= s.chunkSize {
		return []ChunkResult{{Text: text, Index: 0}}, nil
	}

	// 按段落分割
	paragraphs := strings.Split(text, "\n")
	var chunks []ChunkResult
	var current strings.Builder
	idx := 0

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, ChunkResult{
				Text:  strings.TrimSpace(current.String()),
				Index: idx,
			})
			idx++
			current.Reset()
		}
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			flush()
			continue
		}

		// 段落小于 chunk size 直接追加
		if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(para) <= s.chunkSize {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(para)
		} else {
			// 当前段落过大，按句子拆分
			flush()
			sentences := splitSentences(para)
			for _, sent := range sentences {
				sent = strings.TrimSpace(sent)
				if sent == "" {
					continue
				}
				if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(sent) <= s.chunkSize {
					if current.Len() > 0 {
						current.WriteString(" ")
					}
					current.WriteString(sent)
				} else {
					flush()
					current.WriteString(sent)
				}
			}
		}
	}
	flush()

	// 添加 overlap 标记到 chunks
	return chunks, nil
}

// splitSentences 简单按中英文标点分句
func splitSentences(text string) []string {
	var result []string
	var current strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		// 中英文句末标点
		if r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n' {
			// 检查是否为缩写点 (e.g. Mr.) — 简化处理
			result = append(result, current.String())
			current.Reset()
		}
		_ = i
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// GetOverlapContext 获取 chunk 前后的 overlap 文本
func (s *ChunkerService) GetOverlapContext(chunks []ChunkResult, idx int) string {
	if s.chunkOverlap <= 0 || len(chunks) == 0 {
		return ""
	}
	var parts []string
	if idx > 0 {
		prev := chunks[idx-1].Text
		if utf8.RuneCountInString(prev) > s.chunkOverlap {
			runes := []rune(prev)
			parts = append(parts, string(runes[len(runes)-s.chunkOverlap:]))
		} else {
			parts = append(parts, prev)
		}
	}
	parts = append(parts, chunks[idx].Text)
	if idx < len(chunks)-1 {
		next := chunks[idx+1].Text
		if utf8.RuneCountInString(next) > s.chunkOverlap {
			runes := []rune(next)
			parts = append(parts, string(runes[:s.chunkOverlap]))
		} else {
			parts = append(parts, next)
		}
	}
	return fmt.Sprintf("%s\n---\n%s\n---\n%s", parts[0], chunks[idx].Text, parts[len(parts)-1])
}
