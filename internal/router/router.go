package router

import (
	"log"
	"rag-engine-go/internal/config"
	"rag-engine-go/internal/handler"
	"rag-engine-go/internal/service"

	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

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

	parser := service.NewParserService(cfg.TikaURL)
	chunker := service.NewChunkerService(cfg.ChunkSize, cfg.ChunkOverlap)
	embedder := service.NewEmbedderService(cfg.OllamaURL, cfg.EmbeddingModel)
	indexer, err := service.NewIndexerService(cfg)
	if err != nil {
		log.Printf("[WARN] ES unreachable at %s — search endpoints will return 503", cfg.ESURL())
	}

	if indexer != nil && !indexer.IndexExists() {
		if err := indexer.CreateIndex(); err != nil {
			log.Printf("[WARN] ES index creation failed: %v — search may fail for dense_vector queries", err)
		}
	}

	var searcher *service.SearcherService
	if indexer != nil {
		searcher = service.NewSearcherService(indexer.ES(), cfg.ESIndex)
	}

	qa := service.NewQAService(cfg.LLMProvider, cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	docService := service.NewDocumentService(parser, chunker, embedder, indexer, searcher, qa)

	uploadH := handler.NewUploadHandler(docService)
	searchH := handler.NewSearchHandler(docService)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			esOK := indexer != nil
			c.JSON(200, gin.H{
				"status": "ok",
				"tika":   parser.Health(),
				"ollama": embedder.Health(),
				"es":     esOK,
				"qa":     qa.IsEnabled(),
			})
		})
		api.GET("/stats", uploadH.Stats)
		api.POST("/documents/upload", uploadH.Upload)
		api.GET("/documents", uploadH.ListDocs)
		api.DELETE("/documents/:doc_id", uploadH.DeleteDoc)
		api.POST("/search", searchH.Search)
		api.POST("/qa", searchH.QA)
		api.POST("/qa/stream", searchH.QAStream)
	}

	r.GET("/", serveUI)
	return r
}
