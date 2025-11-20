# giai

实验性的 Go 版 Agent 框架脚手架，参考 langchain / autogen 等设计思路，便于后续扩展模型、工具、记忆等模块。

## 目录结构

- `cmd/giai/`：示例入口，演示最小可运行代理。
- `pkg/agent/`：Agent 组装、运行逻辑。
- `pkg/provider/`：LLM Provider 接口与具体实现（OpenAI、Echo 等）。
- `pkg/tool/`：工具接口与函数适配器。
- `pkg/memory/`：会话记忆接口与内存实现。
- `pkg/prompt/`：轻量模板工具。
- `pkg/types/`：基础类型（消息等）。

## 快速开始

1. 确保已安装 Go 1.21+。
2. 根据需要修改 `go.mod` 中的 module 名称（默认 `giai`）。
3. 运行示例：

```bash
go run ./cmd/giai
```

## 使用 OpenAI 模型

- 填好环境变量：`OPENAI_API_KEY`（必需），可选 `OPENAI_MODEL`、`OPENAI_BASE_URL`（兼容自建网关/Azure）。
- 示例入口会优先使用 OpenAI Provider；未设置 Key 时自动回退到本地回声模型。
- 首次运行请执行 `go mod tidy` 以拉取依赖。

## 使用 OpenRouter 模型

- 填好环境变量：`OPENROUTER_API_KEY`（必需），可选 `OPENROUTER_MODEL`、`OPENROUTER_BASE_URL`（默认 `https://openrouter.ai/api/v1`）、`OPENROUTER_REFERER`、`OPENROUTER_APP_NAME`。
- 示例入口会优先使用 OpenRouter Provider（检测到 `OPENROUTER_API_KEY`），失败后再自动尝试 OpenAI，再回退到本地回声模型。

## 流式输出

- `agent.Agent.RunStream(ctx, input, onDelta)` 支持流式处理，`onDelta` 用于接收增量片段（可为 nil），函数返回最终聚合文本。

## 下一步可以做的

- 接入更多 Provider（Azure OpenAI、Ollama、Bedrock 等），复用统一接口。
- 增加基于工具选择/计划的决策层，封装自动调用 `Agent.UseTool`。
- 扩展记忆（向量库、KV 存储）与持久化。
- 加入 tracing/metrics、中间件等工程化能力。
