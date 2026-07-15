package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ParserService — Apache Tika 文档解析
// 调用 Tika Server REST API: PUT /tika
type ParserService struct {
	tikaURL   string
	client    *http.Client
}

func NewParserService(tikaURL string) *ParserService {
	return &ParserService{
		tikaURL: tikaURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Parse 通过 Tika 提取文档文本内容
func (s *ParserService) Parse(file multipart.File, filename string) (string, error) {
	req, err := http.NewRequest("PUT", s.tikaURL+"/tika", file)
	if err != nil {
		return "", fmt.Errorf("tika request: %w", err)
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tika unreachable (start with: docker run -d -p 9998:9998 apache/tika): %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("tika read: %w", err)
	}
	return string(content), nil
}

// Health 检查 Tika 是否在线
func (s *ParserService) Health() bool {
	resp, err := s.client.Get(s.tikaURL + "/tika")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
