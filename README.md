# 📚 RAG 文档问答引擎

基于 **Go 语言** 的智能文档检索与问答系统。上传 PDF/DOCX/TXT，自动解析 → 分块 → 索引 → 语义搜索 → AI 问答。

![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## 💡 一句话说清

**传统搜索**：搜"消防合同" → 只匹配包含"消防合同"四个字的文档 → 漏掉"消防安全改造协议"

**RAG 语义搜索**：搜"去年消防那边的合同" → 理解语义 → 精准命中"2024年消防安全改造协议书"

## 🎯 核心特性

| 能力 | 实现 |
|------|------|
| 🔍 **混合检索** | BM25 关键词 + 向量语义搜索，双路召回 |
| 📄 **智能解析** | Apache Tika 解析 PDF/DOCX/HTML 等上百种格式 |
| 🧠 **本地 Embedding** | Ollama BGE-M3 生成 1024 维文本向量，零 API 费用 |
| 💬 **AI 问答** | 检索结果 + LLM → 流式生成回答（SSE），支持 Ollama/Claude/OpenAI |
| 🔪 **智能分块** | 滑动窗口文本切割 + 句边界检测 + overlap 上下文 |
| ⚡ **零依赖运行** | 无 ES/Ollama/Tika 时自动降级为纯 Go 本地搜索引擎 |
| 🌐 **内嵌 Web UI** | 深色主题界面，上传→搜索→问答全流程可视化 |

## 🚀 快速开始

### 零依赖模式（推荐先体验）

```bash
git clone https://github.com/Mapl1n/rag-engine-go.git
cd rag-engine-go
go run ./cmd/server
# 打开 http://localhost:8081
```

一条命令，无需任何外部服务。上传 .txt 文件即可体验搜索。

### 完整模式（语义检索 + AI 问答）

```bash
# 一键启动全部依赖
docker compose up -d

# 拉取 Embedding 模型
ollama pull bge-m3

# 启动 Go 应用
go run ./cmd/server
```

## 📡 API 端点

### 文档管理
```
POST /api/documents/upload   上传文档（multipart/form-data）
GET  /api/documents          列出已上传文档
GET  /api/stats              系统统计信息
```

### 检索 & 问答
```
POST /api/search             语义搜索 { "query": "...", "top_k": 5 }
POST /api/qa                 同步问答   { "question": "...", "top_k": 5 }
POST /api/qa/stream          流式问答（SSE）→ 逐 token 返回
```

### 健康检查
```
GET  /api/health             服务状态（ES/Tika/Ollama/LLM 是否在线）
```

## 🏗️ 架构

```
用户 → 上传 PDF/DOCX
       ↓
   Tika 解析 → 纯文本
       ↓
   智能分块 → [chunk1, chunk2, ..., chunkN]
       ↓
   Ollama Embedding → 1024-dim 向量
       ↓
   ElasticSearch [dense_vector + BM25]
       ↓
用户 → 自然语言提问
       ↓
   Embedding 查询向量 + BM25 关键词 → 混合检索
       ↓
   检索结果 + prompt → LLM → 流式回答
```

## 🔧 技术栈

| 组件 | 用途 | 降级方案 |
|------|------|---------|
| **Apache Tika** | 文档解析 | 直接读取 .txt |
| **Ollama BGE-M3** | 文本向量化 | 本地词频向量 |
| **ElasticSearch 8.x** | 向量 + 全文检索 | 内存 BM25 引擎 |
| **LLM (Ollama/Claude)** | AI 回答 | 返回检索原文 |

---

## 📝 License

MIT
