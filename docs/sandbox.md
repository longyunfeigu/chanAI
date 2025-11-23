# Sandbox 设计与使用指南

> 目标：让工具“只依赖接口，不直接碰真实系统”，把所有有副作用的操作（读写文件、执行命令）统一收敛到一个可控的沙箱层。
>
> 这篇文档是对 `docs/design/tool.md` 的补充，聚焦在“执行环境 / 文件系统”的抽象上。

---

## 1. 为什么需要 Sandbox

在没有 Sandbox 的情况下，工具经常这样写：

```go
data, err := os.ReadFile(path)
cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
```

问题有：

- 工具直接依赖真实系统（当前这台机器的文件系统和 shell），
  - 想迁移到 Docker / 远程沙箱 / 测试环境时，需要逐个工具重写。
- 安全策略分散：
  - 每个工具都要自己想路径校验、危险命令过滤，一不小心就漏掉。
- 难以测试：
  - 单测会真的去读写磁盘、执行命令，速度慢、不稳定，还容易破坏环境。

**Sandbox 的核心思想**：

- 把“文件 / 命令”相关的能力封装成一个接口 `Sandbox`，
- 工具只调用接口（`tc.Sandbox.FS().Read` / `tc.Sandbox.Exec`），
- 真正跟系统打交道的，是 Sandbox 的各种实现（本地、远程、Mock 等）。

这和 `docs/design/tool.md` 里的理念是一致的：

- Tool 只关注“做什么”（输入 / 输出 / 业务逻辑），
- 如何调度、如何限时、如何重试交给 `Executor`，
- 如何访问系统资源交给 `Sandbox`。

---

## 2. 核心接口：Sandbox / SandboxFS

在 `pkg/sandbox/interface.go` 中可以看到两个关键接口：

### 2.1 `SandboxFS`：受控的文件系统视图

```go
type SandboxFS interface {
    Resolve(path string) string
    IsInside(path string) bool
    Read(ctx context.Context, path string) (string, error)
    Write(ctx context.Context, path string, content string) error
    Temp(name string) string
    Stat(ctx context.Context, path string) (FileInfo, error)
    Glob(ctx context.Context, pattern string, opts *GlobOptions) ([]string, error)
}
```

设计重点：

- **路径解析与边界检查**：
  - `Resolve` 负责把相对路径转换为工作目录下的绝对路径。
  - `IsInside` 决定一个路径是否在沙箱允许的范围内，避免路径逃逸。
- **统一的读写入口**：
  - Tool 不直接操作 `os`，统一通过 `Read/Write/Stat/Glob` 访问文件。
- **扩展性**：
  - `GlobOptions` 支持 ignore 列表、是否返回绝对路径、是否包含隐藏文件等。

### 2.2 `Sandbox`：对工具暴露的完整执行环境

```go
type Sandbox interface {
    Kind() string
    WorkDir() string
    FS() SandboxFS
    Exec(ctx context.Context, cmd string, opts *ExecOptions) (*ExecResult, error)
    Watch(paths []string, listener FileChangeListener) (string, error)
    Unwatch(watchID string) error
    Dispose() error
}
```

对应能力：

- `FS()`：拿到前面说的沙箱文件系统。
- `Exec`：在受控环境中执行命令，返回：
  - `Code`：退出码。
  - `Stdout` / `Stderr`：输出。
- `Watch` / `Unwatch`：
  - 面向需要监听文件变化的高级工具，内部用 `fsnotify` 做实现。
- `Dispose`：
  - 用于释放资源（关闭 watcher、清理远程会话等）。

从 Tool 的视角看，**这就是整个“系统世界”**：

- 不关心当前是在本地机器、Docker 容器还是远程云端，
- 只关心“我能读文件、写文件、跑命令、监听变更”。

---

## 3. 本地实现：LocalSandbox / LocalFS

本地实现位于：

- `pkg/sandbox/local.go`
- `pkg/sandbox/local_fs.go`

### 3.1 `LocalSandbox`：本地命令执行 + 文件监听

构造：

```go
type LocalSandboxConfig struct {
    WorkDir         string
    EnforceBoundary bool
    AllowPaths      []string
    WatchFiles      bool
}

func NewLocalSandbox(config *LocalSandboxConfig) (*LocalSandbox, error)
```

要点：

- `WorkDir`：
  - 沙箱的根目录，未指定时自动取 `os.Getwd()`。
  - 所有相对路径都会基于它解析。
- `EnforceBoundary`：
  - 为 true 时，路径默认不能逃出 `WorkDir`，除非在 `AllowPaths` 白名单中。
- `AllowPaths`：
  - 额外允许访问的目录列表，适合挂一些共享目录（例如 `/tmp`）。
- `WatchFiles`：
  - 是否启用文件变更监听能力。

命令执行（`Exec`）逻辑中做了几件事情：

- **危险命令过滤**：
  - 内置了一组正则，如：
    - `rm -rf /`
    - `sudo ...`
    - `curl ... | bash`
    - `dd ... of=/dev/sda`
  - 匹配到则直接返回 `ExecResult`，不真正执行命令。
- **超时控制**：
  - 默认 120s，可通过 `ExecOptions.Timeout` 覆盖。
- **工作目录 & 环境变量**：
  - `WorkDir` 默认为 `LocalSandbox.workDir`。
  - 可通过 `ExecOptions.WorkDir` 和 `ExecOptions.Env` 覆盖。

### 3.2 `LocalFS`：带边界的本地文件系统

构造在 `NewLocalSandbox` 中完成，关键逻辑在：

- 路径判断：

  ```go
  // 先看是否在 workDir 内
  relativeToWork, err := filepath.Rel(lfs.workDir, resolved)
  // 若 EnforceBoundary 为 false，则放宽为“永远允许”
  // 否则再检查 AllowPaths 白名单
  ```

- 读写统一做 `IsInside` 检查，超出边界直接报错：

  ```go
  if !lfs.IsInside(resolved) {
      return "", fmt.Errorf("path outside sandbox: %s", path)
  }
  ```

- `Glob` 使用 `doublestar` 实现，支持 `**` 等高级模式，并自动过滤越界路径和 ignore 列表。

**你可以学到的点：**

- 对“有副作用”的操作，默认要严格：先判边界，再做动作。
- 把路径安全、危险命令过滤、超时控制集中到 Sandbox 层，而不是分散到每个 Tool 里。

---

## 4. Mock 实现：MockSandbox / MockFS

位置：`pkg/sandbox/mock.go`

用途：**单元测试**。

- `MockSandbox`：
  - 不真正执行命令，只返回一个可预测的 `ExecResult`。
  - `FS()` 返回内存版的 `MockFS`。
- `MockFS`：
  - 用一个 `map[string]string` 存文件内容。
  - `Read/Write/Stat/Glob` 都在内存中完成。

有了这个实现，你可以在测试里这样构造 `ToolContext`：

```go
sb := sandbox.NewMockSandbox()
tc := tool.NewToolContext(
    tool.WithSandbox(sb),
)
```

然后测试 Tool 时不会真的读写磁盘，也不会执行任何命令，测试更稳定更快。

---

## 5. 工厂模式：按配置创建 Sandbox

位置：`pkg/sandbox/factory.go`

核心类型：

```go
type Kind string

const (
    KindLocal  Kind = "local"
    KindMock   Kind = "mock"
    KindRemote Kind = "remote"
)

type Config struct {
    Kind            Kind
    WorkDir         string
    EnforceBoundary bool
    AllowPaths      []string
    WatchFiles      bool
    Extra           map[string]any
}

type Factory struct{}
```

使用：

```go
factory := sandbox.NewFactory()
sb, err := factory.Create(&sandbox.Config{
    Kind:            sandbox.KindLocal,
    WorkDir:         ".",
    EnforceBoundary: true,
    AllowPaths:      []string{},
    WatchFiles:      true,
})
```

目前支持：

- `KindLocal`：使用 `LocalSandbox`。
- `KindMock`：使用 `MockSandbox`。
- `KindRemote`：返回一个基于 HTTP 的 `RemoteSandbox` 骨架（`Exec`/FS 需要你根据实际远程 API 扩展）。

这个工厂模式和 `docs/design/tool.md` 里 `Registry` 的思路类似：

- Tool 通过工厂和配置创建，实现“按配置切换实现”；
- 上层只需要改配置，不用改具体调用代码。

---

## 6. 与 ToolContext 的集成方式

`ToolContext` 是整个 Tool 系统的“执行上下文”，在 `docs/design/tool.md` 中已经介绍过。  
现在我们为它增加了一个可选的 `Sandbox` 字段：

```go
type ToolContext struct {
    ...
    Logger   Logger
    Storage  Storage
    Sandbox  sandbox.Sandbox
}

func WithSandbox(s sandbox.Sandbox) Option {
    return func(tc *ToolContext) {
        tc.Sandbox = s
    }
}
```

这意味着：

- 创建 ToolContext 时，可以选择性注入 sandbox：

  ```go
  sb, err := sandbox.NewLocalSandbox(&sandbox.LocalSandboxConfig{
      WorkDir:         ".",
      EnforceBoundary: true,
      AllowPaths:      nil,
      WatchFiles:      false,
  })
  if err != nil { ... }

  tc := tool.NewToolContext(
      tool.WithSandbox(sb),
      // 其他 WithXxx...
  )
  ```

- Tool 实现中，如果需要读写文件或执行命令，应优先通过 `tc.Sandbox` 操作：

  ```go
  func (t *SomeTool) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
      if tc == nil || tc.Sandbox == nil {
          return nil, fmt.Errorf("sandbox not configured")
      }

      // 读文件
      content, err := tc.Sandbox.FS().Read(ctx, "README.md")

      // 执行命令
      res, err := tc.Sandbox.Exec(ctx, "ls -la", &sandbox.ExecOptions{Timeout: 30 * time.Second})
      ...
  }
  ```

这样就实现了文档开头那句目标：

> 工具只依赖接口 (`Sandbox` / `SandboxFS`)，不直接碰真实系统。

---

## 7. 迁移建议：从直接系统调用到 Sandbox

如果你想把现有工具逐步迁移到使用 Sandbox，可以按下面的顺序：

1. **给 ToolContext 注入 Sandbox（已完成）**
   - 确保创建 `ToolContext` 的地方都能选择性传入 Sandbox 实例。

2. **从不敏感工具开始迁移**
   - 例如：代码搜索 / 文件读取工具（`read_file` / `glob` / `grep`）：
     - 先写一个新版实现，优先使用 `tc.Sandbox.FS()`，如果为 nil 再回退到 `os`。
   - 保持对旧行为的兼容，在没有配置 Sandbox 时仍能工作。

3. **最后迁移有副作用的工具**
   - 比如 `bash` 这类执行命令的工具。
   - 迁入 Sandbox 后，可以完全依赖 `LocalSandbox` 的危险命令过滤和超时控制。

4. **单测使用 MockSandbox**
   - 对于依赖文件 / 命令的工具，测试时使用 `NewMockSandbox()`，确保测试环境不会真的改系统。

---

## 8. 总结：Tool + Executor + Sandbox 的分层

结合 `docs/design/tool.md` 和本篇 Sandbox 设计，可以看到这一套分层：

- **Tool**：描述能力（名字、Schema、Execute）；不关心“如何调度”和“如何访问系统”。
- **Executor**：负责并发、超时、重试、审批逻辑（见 `pkg/tool/executor.go`）。
- **Sandbox**：负责文件系统、命令执行、安全边界（见 `pkg/sandbox`）。

三者配合的效果：

- Tool 的实现简洁，只关心业务与输入输出；
- 安全问题集中到 Sandbox 层处理（路径、命令、权限）；
- 稳定性问题集中到 Executor 层处理（超时、重试、优先级）。

当你在别的项目里需要“执行任务 + 调用外部系统”时，可以直接复用这套思路：

- 先设计接口（Tool / Sandbox），
- 再用适配器和工厂做实现切换，
- 最后通过 Executor 把执行策略和业务逻辑解耦。 

