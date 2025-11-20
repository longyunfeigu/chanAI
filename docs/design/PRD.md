# GIAI (Go Intelligent Agent Interface) - Product Requirement Document

## 1. 项目愿景 (Vision)
构建一个 **Go Native** 的 AI Agent SDK，旨在结合 LangChain 的工具生态、AutoGPT 的自主循环能力以及 Semantic Kernel 的规划理念。利用 Go 语言的高并发、强类型和编译时检查特性，提供比 Python 框架更稳定、更高性能的生产级 Agent 开发体验。

## 2. 核心架构 (Architecture)

采用 "洋葱圈" 架构设计，由内向外：

1.  **Protocol Layer (协议层)**: 定义标准化的 `Message`, `ChatModel`, `Embedding`, `VectorStore` 接口。
2.  **Kernel Layer (核心层)**: 提示词引擎 (Prompt Engine)、输出解析 (Output Parsers)、记忆管理 (Memory)。
3.  **Capability Layer (能力层)**: 工具系统 (Tools)、规划器 (Planners/ReAct)、RAG 引擎。
4.  **Agent Layer (代理层)**: 封装好的特定用途 Agent (e.g., `ConversationalAgent`, `PlanAndExecuteAgent`)。
5.  **Application Layer (应用层)**: REST API, CLI, gRPC 服务。

---

## 3. 阶段规划 (Phased Roadmap)

### 阶段一：核心基础 (Foundation)
**目标**: 建立统一的模型交互层，屏蔽底层 Provider (OpenAI, Anthropic, Local LLM) 的差异。

*   **1.1 统一消息模型 (Unified Message System)**
    *   定义 `Message` 接口及 `UserMessage`, `AIMessage`, `SystemMessage`, `ToolMessage` 实现。
    *   支持多模态内容 (Text + Image)。
*   **1.2 模型接口 (Model Interfaces)**
    *   `ChatModel`: 支持 `Generate`, `Stream` (利用 Go Channel)。
    *   `EmbeddingModel`: 支持 `EmbedDocuments`, `EmbedQuery`。
*   **1.3 Provider 适配**
    *   重构现有的 OpenAI 实现。
    *   增加配置管理 (`Options` pattern)。

### 阶段二：能力与工具 (Capabilities & Tools)
**目标**: 让 Agent 具备执行外部操作的能力，并能结构化输出结果。

*   **2.1 工具系统 (Tool System)**
    *   定义 `Tool` 接口：包含 Schema (`Name`, `Description`, `Parameters`) 和 `Run` 方法。
    *   **Reflection Tool**: 支持通过 Go Struct 标签自动生成 JSON Schema。
*   **2.2 结构化输出 (Structured Output)**
    *   实现 `OutputParser`，支持将 LLM 的 JSON 文本响应自动 Unmarshal 为 Go Struct。
    *   错误修复机制：当解析失败时自动重试。
*   **2.3 基础链 (Chains)**
    *   `LLMChain`: 提示词 + 模型 + 解析器的最简组合。

### 阶段三：记忆与知识 (Memory & RAG)
**目标**: 实现上下文管理与外部知识库检索。

*   **3.1 记忆组件 (Memory)**
    *   `ChatHistory`: 抽象存储接口 (In-Memory, Redis, BoltDB)。
    *   `WindowMemory`: 基于 Token 限制或消息数量的滑动窗口记忆。
*   **3.2 RAG 基础**
    *   `Document`: 文档结构体。
    *   `VectorStore`: 向量数据库统一接口 (Add, Search)。
    *   实现简单的内存向量库用于测试。

### 阶段四：自主智能体 (Autonomous Agents)
**目标**: 实现具备规划能力的 Agent。

*   **4.1 规划器 (Planner)**
    *   实现 **ReAct** (Reasoning + Acting) 循环模式。
    *   支持 `Step` 级别的 Hook，用于观察思考过程。
*   **4.2 代理运行时 (Agent Runtime)**
    *   设计 `AgentExecutor`，管理思考 -> 工具调用 -> 结果回填 -> 再思考 的循环。
    *   支持并发工具执行 (`worker pool` pattern)。

### 阶段五：生产级特性 (Production Ready)
**目标**: 可观测性与服务化。

*   **5.1 可观测性 (Observability)**
    *   集成 OpenTelemetry。
    *   支持回调机制 `Callbacks`，用于日志记录和调试。
*   **5.2 服务化 (Serving)**
    *   提供标准 HTTP/gRPC 接口封装。

---

## 4. 接口设计草案 (Interface Draft)

### LLM Interface
```go
type ChatModel interface {
    // 同步调用
    Chat(ctx context.Context, messages []Message, options ...Option) (*ChatResponse, error)
    // 流式调用
    Stream(ctx context.Context, messages []Message, options ...Option) (<-chan ChatChunk, error)
}
```

### Tool Interface
```go
type Tool interface {
    Name() string
    Description() string
    Schema() string // JSON Schema
    Run(ctx context.Context, input string) (string, error)
}
```

### Agent Interface
```go
type Agent interface {
    Run(ctx context.Context, input string) (string, error)
}
```

