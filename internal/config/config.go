package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServerPort string
	AppEnv     string

	// ElasticSearch
	ESHost  string
	ESPort  string
	ESIndex string

	// Tika
	TikaURL string

	// Ollama (Embedding)
	OllamaURL          string
	EmbeddingModel     string

	// Minio
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// LLM (optional, pluggable)
	LLMProvider string // ollama | openai | claude
	LLMBaseURL  string
	LLMAPIKey   string
	LLMModel    string

	// Chunking
	ChunkSize    int
	ChunkOverlap int
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8081"),
		AppEnv:         getEnv("APP_ENV", "development"),
		ESHost:         getEnv("ES_HOST", "localhost"),
		ESPort:         getEnv("ES_PORT", "9200"),
		ESIndex:        getEnv("ES_INDEX", "documents"),
		TikaURL:        getEnv("TIKA_URL", "http://localhost:9998"),
		OllamaURL:      getEnv("OLLAMA_URL", "http://localhost:11434"),
		EmbeddingModel: getEnv("EMBEDDING_MODEL", "bge-m3"),
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "documents"),
		MinioUseSSL:    getEnvBool("MINIO_USE_SSL", false),
		LLMProvider:    getEnv("LLM_PROVIDER", ""),
		LLMBaseURL:     getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:      getEnv("LLM_API_KEY", ""),
		LLMModel:       getEnv("LLM_MODEL", "qwen2.5:7b"),
		ChunkSize:      getEnvInt("CHUNK_SIZE", 512),
		ChunkOverlap:   getEnvInt("CHUNK_OVERLAP", 50),
	}
}

func (c *Config) ESURL() string {
	return fmt.Sprintf("http://%s:%s", c.ESHost, c.ESPort)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1"
	}
	return fallback
}
