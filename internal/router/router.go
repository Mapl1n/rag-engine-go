package router

import (
	"rag-engine-go/internal/config"
	"rag-engine-go/internal/handler"
	"rag-engine-go/internal/service"

	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// ── Initialize Services ──
	parser := service.NewParserService(cfg.TikaURL)
	chunker := service.NewChunkerService(cfg.ChunkSize, cfg.ChunkOverlap)
	embedder := service.NewEmbedderService(cfg.OllamaURL, cfg.EmbeddingModel)
	indexer, err := service.NewIndexerService(cfg)
	if err != nil {
		panic("ES: " + err.Error())
	}

	// Auto-create ES index
	if !indexer.IndexExists() {
		indexer.CreateIndex()
	}

	searcher := service.NewSearcherService(indexer.ES(), cfg.ESIndex)
	qa := service.NewQAService(cfg.LLMProvider, cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	docService := service.NewDocumentService(parser, chunker, embedder, indexer, searcher, qa)

	// ── Handlers ──
	uploadH := handler.NewUploadHandler(docService)
	searchH := handler.NewSearchHandler(docService)

	// ── API Routes ──
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "ok",
				"tika":    parser.Health(),
				"ollama":  embedder.Health(),
				"qa":      qa.IsEnabled(),
			})
		})

		api.GET("/stats", uploadH.Stats)

		// 文档上传
		api.POST("/documents/upload", uploadH.Upload)
		api.GET("/documents", uploadH.ListDocs)

		// 检索
		api.POST("/search", searchH.Search)

		// 问答
		api.POST("/qa", searchH.QA)
		api.POST("/qa/stream", searchH.QAStream)
	}

	// ── Web UI ──
	r.GET("/", serveUI)

	return r
}
