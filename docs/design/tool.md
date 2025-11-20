# Tool 设计理念与实战教程

> 目标：让一个刚写过几百行 Go 的同学，看完这个文档以后，不只是“能用 `pkg/tool`”，而是学会一套可以在别的项目里复用的设计方法和编码套路。

## 0. 从“只会写函数”到“会设计 Tool 系统”

### 0.1 最原始的写法

假设你在做一个聊天机器人，需要它“帮我搜一下代码”“帮我读一个文件”。最直接的写法通常是：

```go
func SearchCode(pattern, path string) (string, error) { ... }
func ReadFile(path string) (string, error) { ... }
```

模型调用时，你可能会写一堆 if/else：

```go
switch toolName {
case "search_code":
    return SearchCode(pat, path)
case "read_file":
    return ReadFile(path)
}
```

这样写当然能跑，但问题有：

- 每加一个工具，就要改一堆 if/else。
- 没有统一的“工具接口”，无法做统一的日志、超时、重试。
- 无法给工具自动生成 Schema，让 LLM 按约定格式调用。

接下来就是这个包要解决的问题：**升级思维，从“堆函数”变成“设计可扩展的工具系统”**。

---

## 1. 核心抽象：Tool / EnhancedTool

### 1.1 接口设计：先想“别人怎么用”

打开 `pkg/tool/types.go`，你能看到：

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error)
    Prompt() string
}
```

这里体现了几个重要的设计原则：

- **接口只表达“做什么”，不关心“怎么做”**：只关心“执行时需要什么信息”（名字/描述/输入 Schema/执行函数），不暴露内部细节。
- **接口粒度适中**：如果一开始就把超时、重试、审批、优先级等都塞进 `Tool`，接口会非常臃肿；这里把基础能力放在 `Tool`，高级能力放到 `EnhancedTool`。

`EnhancedTool` 是在 `Tool` 上的加法：

```go
type EnhancedTool interface {
    Tool
    IsLongRunning() bool
    Timeout() time.Duration
    Priority() int
    RequiresApproval() bool
    RetryPolicy() *RetryPolicy
}
```

**你可以学到的点：**

- 当你不确定未来是否一定需要某些能力（比如审批、优先级），可以用“基础接口 + 扩展接口”的方式，而不是把所有字段硬塞到一个接口里。
- 在别的项目里，如果你有“普通任务”和“高级任务”，也可以这样设计：`Job` + `AdvancedJob`。

---

## 2. 组合优于继承：BaseTool 的用法

看 `pkg/tool/base.go`：

```go
type BaseTool struct {
    NameVal        string
    DescVal        string
    SchemaVal      map[string]any
    PromptVal      string
    IsLongRunningVal    bool
    TimeoutVal          time.Duration
    PriorityVal         int
    RequiresApprovalVal bool
    RetryPolicyVal      *RetryPolicy
}
```

以及一堆 getter：

```go
func (b *BaseTool) Name() string          { return b.NameVal }
func (b *BaseTool) Description() string   { return b.DescVal }
...
```

这里用的是 Go 里非常经典的实践：

- **用结构体 + 组合（embedding），而不是继承**。
- 每个具体工具只需要嵌入 `BaseTool`，就天然实现了 `Tool` / `EnhancedTool` 的大部分方法。

例如在 `builtin/bash.go`：

```go
type Bash struct {
    tool.BaseTool
}
```

然后只实现 `Execute` 即可。这种模式非常适合：

- 你有一堆结构体都需要“同一套字段 + 部分相同行为”时。
- 不想每个工具都重复写 Name/Description/Timeout 这些 getter 时。

**你可以学到的点：**

- 以后遇到类似“有几十种 xxx，但它们有一堆公共字段/行为”的场景时，可以先抽一个 `BaseXXX`，子类通过 embedded struct 复用它。
- Base 里只放“无业务逻辑的字段/简单方法”，不要放具体的业务逻辑，避免 Base 变成“上帝类”。

---

## 3. 适配器模式：把普通函数变成 Tool

很多时候业务方已经有现成的函数，不想为了适配框架重新写一遍。这时最好的做法是——写一个“适配器”（Adapter）。

### 3.1 `Func` 适配器

看 `pkg/tool/adapter.go` 中：

```go
type Callable func(ctx context.Context, input map[string]any, tc *ToolContext) (any, error)

type Func struct {
    BaseTool
    fn Callable
}
```

配合构造函数：

```go
func NewFunc(name, description string, fn Callable) *Func {
    f := &Func{
        BaseTool: NewBaseTool(name, description),
        fn:       fn,
    }
    // 默认 schema
    f.SchemaVal = map[string]any{ ... }
    return f
}
```

以及：

```go
func (f *Func) Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error) {
    if f.fn == nil {
        return nil, fmt.Errorf("tool %s has no implementation", f.Name())
    }
    return f.fn(ctx, input, tc)
}
```

**这就是典型的“适配器模式”**：

- 原本你只有一个普通函数 `fn(ctx, input, tc)`。
- 通过 `NewFunc`，你把它“适配”成了一个实现 `Tool` 接口的对象。

在自己的项目中，你也可以用这个套路：

- 有一堆函数签名比较统一（比如 `func(ctx, req) (resp, error)`），就可以做一个适配器，把它们统一暴露成更通用的接口或 HTTP Handler等。

### 3.2 `Struct[T]` 适配器：让 Map 变成强类型

普通工具的入参是 `map[string]any`，类型安全很差。`Struct[T]` 提供了一种折中方案：

```go
type Struct[T any] struct {
    BaseTool
    fn func(context.Context, T, *ToolContext) (any, error)
}
```

构造函数：

```go
func NewStruct[T any](name, description string, fn func(context.Context, T, *ToolContext) (any, error)) *Struct[T] {
    var zero T
    s := &Struct[T]{ BaseTool: NewBaseTool(name, description), fn: fn }
    s.SchemaVal = GenerateSchema(zero)
    return s
}
```

核心思想：

- 外部协议（LLM/JSON）用的是 `map[string]any`。
- 你内部写逻辑时，更想用强类型的 struct。
- 用 `Struct[T]` + `GenerateSchema` + `json.Unmarshal`，就可以在边界上把 map 转成 T：

```go
func (s *Struct[T]) Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error) {
    var args T
    raw, _ := json.Marshal(input)
    _ = json.Unmarshal(raw, &args)
    return s.fn(ctx, args, tc)
}
```

**你可以学到的点：**

- 对外不稳定 / 动态结构（map[string]any） + 对内强类型 struct，是一个非常通用的模式。
- 中间用 JSON 做桥接，是一个简单但好用的方案（性能不是极致，但足够实用）。

---

## 4. 执行器 Executor：把“乱七八糟的调用”变成“有策略的执行”

现在再看 `pkg/tool/executor.go`。一个 naive 的执行可能只是：

```go
output, err := tool.Execute(ctx, input, tc)
```

但项目里真正需要的是：

- 限制最多并发多少个工具。
- 每个工具有默认超时，有些工具可以自定义超时。
- 某些错误可以自动重试（带退避）。
- 有的工具需要审批才能跑。
- 一批工具要按优先级排队执行。

这些都集中在 `Executor` 里：

```go
type Executor struct {
    config    ExecutorConfig
    semaphore chan struct{}
}
```

### 4.1 并发控制：信号量模式

```go
select {
case e.semaphore <- struct{}{}:
    defer func() { <-e.semaphore }()
case <-ctx.Done():
    // 上下文取消
}
```

这是 Go 里一个非常经典的并发限制技巧：

- `semaphore` 是一个有缓冲 channel，容量 = 最大并发数。
- 每次执行前往里写一个空 struct，执行完再读出来。
- 如果写不进去，就说明已有太多并发，等待或马上返回。

**你可以学到的点：** 将来只要有“限制最大并发”的需求，都可以用这个模式，不一定非得在这个项目里。

### 4.2 超时控制：context.WithTimeout

```go
execCtx := ctx
var cancel context.CancelFunc
if timeout > 0 {
    execCtx, cancel = context.WithTimeout(ctx, timeout)
}
output, execErr = req.Tool.Execute(execCtx, req.Input, req.Context)
if cancel != nil {
    cancel()
}
```

技巧：

- 不要在函数内部直接用 `context.Background()`，那样无法被上游取消。
- 永远复用上游传进来的 ctx，再叠加自己的超时。

### 4.3 重试与退避

```go
maxAttempts := 1
if retryPolicy != nil {
    maxAttempts += retryPolicy.MaxRetries
}

for attempt := 0; attempt < maxAttempts; attempt++ {
    ...
    if execErr == nil { break }
    if attempt < maxAttempts-1 && isRetryable(execErr, retryPolicy) {
        delay := calculateBackoff(attempt, retryPolicy)
        select {
        case <-time.After(delay):
        case <-ctx.Done():
            execErr = ctx.Err(); break
        }
    }
}
```

`calculateBackoff` 使用指数退避（常见的重试策略）：

```go
backoff := float64(policy.InitialBackoff) * math.Pow(policy.BackoffMultiplier, float64(attempt))
if backoff > float64(policy.MaxBackoff) { backoff = float64(policy.MaxBackoff) }
```

**你可以学到的点：**

- 重试逻辑不要写死在工具内部，而是放在统一的执行层，否则每个工具都要自己复制粘贴一遍。
- 指数退避（初始延迟 * 倍数^attempt）是通用模式，你可以在 HTTP 客户端、任务重试等场景复用。

### 4.4 审批与优先级

审批：

```go
if requiresApproval && !approved(req.Context) {
    return &ExecuteResult{ Success: false, Error: fmt.Errorf("... requires approval ...") }
}
```

审批信息来自 `ToolContext.Metadata["approved"]`，这是一个典型的“**元数据 + 策略解耦**”的做法：

- 执行器不关心审批是怎么来的，只看一个布尔值。
- 上层系统可以决定：是用户点按钮批准，还是通过某些规则自动批准。

优先级（批量执行时）：

```go
type prioritized struct {
    idx int
    req *ExecuteRequest
    priority int
}

sort.SliceStable(items, func(i, j int) bool {
    return items[i].priority > items[j].priority
})
```

**你可以学到的点：**

- 将“与业务无关但对稳定性很重要”的逻辑统一抽到 Executor；以后你写别的系统，也可以抽一个执行层做同样的事。

---

## 5. Registry：工具的“服务发现”

`pkg/tool/registry.go` 提供了一套工具注册与查找机制。

为什么不用一个全局 map？

```go
var tools = map[string]Tool{}
```

因为你会遇到：

- 并发安全问题（需要加锁）。
- 无法支持“工厂模式”（按配置创建新实例）。
- 无法在不同 Agent 之间使用不同工具集。

Registry 的设计：

```go
type Registry struct {
    mu        sync.RWMutex
    factories map[string]ToolFactory
    instances map[string]Tool
}
```

关键点：

- 读多写少场景用 `RWMutex`，提高并发读性能。
- 同时支持：
  - 预构建实例（适合无状态单例工具）。
  - 工厂函数（`ToolFactory`），按配置创建工具。

你可以把它类比成一个简单的 DI 容器或者“服务注册中心”。

**你可以学到的点：**

- 不要害怕为资源管理专门写一个 Registry/Manager，只要职责单一，它就会很好维护。
- 读多写少的 map，习惯性地考虑 `sync.RWMutex`，读路径走 RLock。

---

## 6. ToolContext：用依赖注入取代全局变量

很多新手的写法是：

```go
var globalLogger = log.New(...)

func someTool() {
    globalLogger.Println("...")
}
```

这样会有几个问题：

- 很难在测试里替换 logger。
- 多个 Agent 或请求想用不同日志行为时做不到。

`ToolContext` 的思路是：

```go
type ToolContext struct {
    AgentID     string
    SessionID   string
    ExecutionID string

    Context context.Context
    Metadata map[string]any

    Logger  Logger
    Storage Storage
}
```

配合一组 `WithXxx` Option：

```go
func NewToolContext(opts ...Option) *ToolContext {
    tc := &ToolContext{
        Metadata: make(map[string]any),
        Context:  context.Background(),
    }
    for _, opt := range opts {
        opt(tc)
    }
    return tc
}
```

**你可以学到的点：**

- Option 模式（`func(*T)`）是一种非常通用的可扩展构造方式。
- 以后你有“传一堆可选参数”的需求时，可以考虑：
  - 定义 struct 存放最终配置。
  - 提供 `WithXxx` 函数去修改它。

在别的项目里，你可以用同样的方式给“任务上下文”“请求上下文”注入 logger、追踪 id 等。

---

## 7. Schema & Validate：边界上的“小卫兵”

`pkg/tool/schema.go` 和 `ValidateInput` 的组合是一个典型的“边界校验”模式：

- **GenerateSchema**：从结构体反射生成一个简化版 JSON Schema。
- **ValidateInput**：根据工具提供的 Schema 校验必填字段。

好处：

- 在真正执行工具前，就能把“缺字段”“字段名拼错”这类低级错误提前挡掉。
- 对接 LLM 时，Schema 可以直接给到模型，让它按格式拼参数。

**你可以学到的点：**

- 在“系统边界”做输入校验，而不是在业务函数内部每次重复写。
- Schema 不一定要非常完整，哪怕只负责“类型 + required”，也已经能过滤大量错误。

---

## 8. 编码技巧：从工具实现里能学到什么

这一节从 `builtin` 里的几个工具挑一些有代表性的细节，说明“怎么写更安全、更健壮”的代码。

### 8.1 Grep：处理不同类型的数字输入

在 `grep` 工具里，`context_lines` 可能来自：

- Go 代码直接构造：`map[string]any{"context_lines": 3}` → `int`
- JSON 反序列化：`float64`
- 甚至是 `json.Number`

如果你只写：

```go
if ctxLines, ok := input["context_lines"].(float64); ok { ... }
```

那么从 Go 代码传 `int` 时就会被完全忽略。

这里通过一个小工具函数 `normalizeInt` 把多种常见类型统一转换成 `int`：

```go
func normalizeInt(v any) (int, bool) {
    switch n := v.(type) {
    case int:
        return n, true
    case int64:
        return int(n), true
    case float64:
        return int(n), true
    case json.Number:
        if i, err := n.Int64(); err == nil {
            return int(i), true
        }
    }
    return 0, false
}
```

**你可以学到的点：**

- 在处理外部输入（尤其是反序列化后的 `map[string]any`）时，要有“类型宽容”的意识。
- 写一个小的转换函数，会比在业务逻辑里到处写 type switch 更干净。

### 8.2 Glob：稳定的返回类型

一开始的 Glob 工具返回值有两种情况：

- 正常：`[]string`
- 超过 1000 个时：`map[string]any{ "matches": ..., "total_matches": ... }`

这会给调用方制造麻烦：需要先断言类型再决定怎么解析。

现在调整为统一返回 `GlobResult`：

```go
type GlobResult struct {
    Matches      []string `json:"matches"`
    TotalMatches int      `json:"total_matches"`
    Truncated    bool     `json:"truncated,omitempty"`
    Warning      string   `json:"warning,omitempty"`
}
```

**你可以学到的点：**

- 同一个函数/工具，**返回类型尽量保持稳定**。需要附加信息时，往结构体里加字段，而不是改变顶层类型。
- 如果一定要截断，也要加 `Truncated/Warning` 之类的标志，让调用方有机会做处理。

### 8.3 Bash/ReadFile：安全意识

在 `read_file` 里：

```go
if !filepath.IsAbs(path) {
    return nil, fmt.Errorf("path must be absolute: %s", path)
}
```

在 `bash` 里：

- 限制超时（2 分钟）。
- 捕获 stdout/stderr 和退出码。

**你可以学到的点：**

- 对“有副作用的工具”（读/写文件、执行命令）要默认**保守**：
  - 限制路径（绝对路径、白名单目录）。
  - 限制执行时间。
  - 明确返回错误信息，不要静默失败。

---

## 9. 给自己留几个练习（强烈建议真的写一写）

下面是几个可以在当前项目里就地练习的题目，做完你对这些设计会更有感觉。

1. **实现一个 HTTP GET 工具**
   - 用 `NewStruct` 包装：
     - 字段：`url`（必填）、`headers`（可选 map）、`timeout`（可选）。
   - 在 Execute 里使用 `http.Client`，注意：
     - 继承上游 `ctx`，并且根据参数或工具默认加超时。
     - 限制响应体大小（比如最多 100KB）。

2. **给 Executor 增加“慢调用日志”**
   - 在 `Execute` 结束时，如果 `Duration > 2 * timeout`（或一个固定值），调用 `ToolContext.Logger` 打一行告警。
   - 思考如何在不改动所有工具实现的前提下，完成这个特性。

3. **为 Registry 写单测**
   - 覆盖：
     - `RegisterInstance` + `Get`。
     - `RegisterFactory` + `Create`。
     - `Find` 的大小写不敏感。
   - 在测试里多搞几个 goroutine 并发读，看看是否有 data race（配合 `go test -race`）。

4. **为某个工具补充错误路径用例**
   - 例如：给 `grep` 增加“非法正则表达式”的测试，断言会返回清晰的错误信息。

---

## 10. 总结：可迁移的思维方式

读完这个包，你可以带走的不只是“一个 Tool 框架”，而是一套可迁移的思维方式：

- 用接口定义“别人看到的你”，把内部细节藏起来。
- 用组合和适配器来减少重复，让扩展变得轻松。
- 把“稳定性相关逻辑”（并发、超时、重试、审批、优先级）抽到统一的执行层。
- 在系统边界做输入校验和类型转换，内部尽量用强类型。
- 时刻考虑安全与可观测性：限流、截断、日志、错误信息。

如果你在别的项目里遇到类似场景（比如任务执行、HTTP 调度、批量作业等），完全可以照着这个包的模式，设计你自己的“Tool/Job/Task 系统”。这比死记 API 更能帮你提升编码能力。 
