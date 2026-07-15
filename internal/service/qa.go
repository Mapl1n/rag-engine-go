package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"rag-engine-go/internal/model"
)

// QAService — LLM 问答（可选，可插拔任何 OpenAI 兼容 API）
// 无 LLM 时也能用：仅返回检索结果（RAG 之 R）
type QAService struct {
	provider string // ollama | openai | claude | "" (none)
	baseURL  string
	apiKey   string
	model    string
	client   *http.Client
	enabled  bool
}

func NewQAService(provider, baseURL, apiKey, model string) *QAService {
	s := &QAService{
		provider: provider,
		baseURL:  baseURL,
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: 120 * time.Second},
		enabled:  provider != "",
	}
	if provider == "ollama" && baseURL == "" {
		s.baseURL = "http://localhost:11434/v1"
	}
	return s
}

func (s *QAService) IsEnabled() bool { return s.enabled }

// BuildPrompt 构建 RAG prompt：检索结果 + 用户问题
func (s *QAService) BuildPrompt(question string, results []model.SearchResult) string {
	var context strings.Builder
	for i, r := range results {
		if i >= 5 {
			break // 最多 5 个上下文片段
		}
		fmt.Fprintf(&context, "[文档: %s] %s\n\n", r.DocName, r.Text)
	}

	return fmt.Sprintf(`你是一个专业的文档分析助手。请基于以下文档内容回答问题。
如果文档中没有相关信息，请如实说"文档中未找到相关信息"。

## 文档内容
%s

## 用户问题
%s

## 回答`, context.String(), question)
}

// AskSync 同步问答（OpenAI 兼容 API）
func (s *QAService) AskSync(question string, results []model.SearchResult) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("LLM not configured")
	}

	prompt := s.BuildPrompt(question, results)

	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := map[string]interface{}{
		"model":  s.model,
		"stream": false,
		"messages": []msg{
			{Role: "user", Content: prompt},
		},
	}
	body, _ := json.Marshal(reqBody)

	url := s.baseURL + "/chat/completions"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response")
	}
	return result.Choices[0].Message.Content, nil
}

// AskStream 流式问答（SSE），回调每个 token
func (s *QAService) AskStream(question string, results []model.SearchResult, onToken func(string)) error {
	if !s.enabled {
		return fmt.Errorf("LLM not configured")
	}

	prompt := s.BuildPrompt(question, results)

	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := map[string]interface{}{
		"model":  s.model,
		"stream": true,
		"messages": []msg{
			{Role: "user", Content: prompt},
		},
	}
	body, _ := json.Marshal(reqBody)

	url := s.baseURL + "/chat/completions"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("llm stream: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			onToken("[DONE]")
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil {
			for _, c := range chunk.Choices {
				if c.Delta.Content != "" {
					onToken(c.Delta.Content)
				}
			}
		}
	}
	return nil
}

// SearchOnlyAnswer 仅检索模式（无 LLM）的回答
func SearchOnlyAnswer(results []model.SearchResult) string {
	if len(results) == 0 {
		return "未找到相关文档内容。"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条相关结果：\n\n", len(results)))
	for i, r := range results {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n---\n\n", r.DocName, r.Text[:min(200, len(r.Text))]))
	}
	sb.WriteString("💡 提示：配置 LLM 后可获得智能回答。设置环境变量 LLM_PROVIDER=ollama 即可启用。")
	return sb.String()
}

// FormatRetrievedContext 格式化检索上下文供 LLM 使用
func FormatRetrievedContext(results []model.SearchResult, maxTokens int) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 相关文档内容\n")
	total := 0
	for i, r := range results {
		if total >= maxTokens {
			break
		}
		sb.WriteString(fmt.Sprintf("\n### [来源 %d] %s\n%s\n", i+1, r.DocName, r.Text))
		total += len([]rune(r.Text))
	}
	return sb.String()
}
