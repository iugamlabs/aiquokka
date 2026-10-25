# fork 上游合并指南

本文记录本次同步结果，也作为后续移植的操作规范。历史 `docs/changes/` 文档保留当时背景；实现判断以当前代码、README 和较新的自动恢复规范为准。

## 本次同步基线

- 检查日期：2026-10-25。
- fork：`https://github.com/iugamlabs/aiquokka`；Go module 同名。本地 `origin` 已由旧 `star-plan` 地址改为 `git@github.com:iugamlabs/aiquokka.git`。
- upstream：`https://github.com/McKean/aiquokka.git`，本地远程名 `upstream`。
- 同步前 fork：`698c6e73324afaa940092fa76c2a57d72735e871`。
- 双方共同祖先：`def6ca814d8da68aa36394a189c1bd2d918d3357`（2026-08-28）。
- **已处理到的上游 SHA：`66ec5c129bdaba55d5ee2b14f1e8c2b54fe3c4cd`**（2026-10-23）；本次处理共同祖先之后的 22 个上游提交。
- 采用按功能分类移植，提交正文以 `Upstream-Commit` 记录来源。没有创建 upstream merge，也没有推送、打标签或发布。

分类移植不会推进 Git 的 merge 祖先。GitHub 可能仍显示这 22 个提交未合并，这是预期结果。下次同步使用上面记录的上游 SHA 作为起点，不能再次仅以 `git merge-base` 推断待处理范围。

## fork 规则与架构映射

### 必须保持的约束

1. Provider 实现统一放在 `internal/providers/<id>`，实现共享接口，通过 `catalog.go` 显式注册一次。子命令、聚合、列表、托盘读取同一 Registry；禁止 `init()` 自注册、扫描目录注册或重复 Provider 清单。
2. 默认凭据策略为 `auto`。旧重构方案中的默认 `readonly` 已被 [自动恢复规范](changes/2026-09-12-credential-auto-recovery-optimize.md) 替代；`readonly` 仍是显式选项。
3. 所有命令入口先验证 Policy / Capabilities。刷新和重新登录分开；轮换令牌不能只在内存刷新，持久化使用原子写入并保留其他字段、账户及凭据来源。Grok 保留官方兼容的文件锁，并合并同一次查询中的并发刷新。
4. `--no-refresh` 使用现有 `readonly` 策略，不另建 refresh context。可与显式 `readonly` 同用，与显式其他策略冲突。Kiro / Antigravity / Muse CLI 查询由官方客户端管理其凭据，这不等于禁止客户端自行刷新。
5. 官方交互登录仅允许现有 Codex / Grok 单 Provider、双 TTY、非 watch、非结构化输出且非 CI 的路径；托盘、聚合及脚本不得启动交互登录。
6. 终端和托盘以剩余额度展示：`left = 100 - used_percent`。告警按已用比例触发；节奏标记仍基于原始 `Pace`。缺少权威百分比保持未知，尤其不能根据 Cursor 的消费金额猜额度。
7. 保留 Cursor 的凭据来源绑定、Codex 完整 `reset_credits`、DeepSeek / OpenRouter 事实式金额展示。不得用上游序列化或渲染代码覆盖这些 fork 能力。
8. 默认 CLI 的 Windows / macOS / Linux × amd64 / arm64 六种构建均保持 `CGO_ENABLED=0`。托盘是独立构建，不能为接入桌面功能删除原有发布目标。

| 上游结构或行为 | 本 fork 的接入位置／适配方式 |
| --- | --- |
| `internal/<provider>` | `internal/providers/<provider>`，通过 `New()` 适配共享接口 |
| `cmd/<provider>.go`、旧 `allProviders` 切片 | 由 Registry 生成子命令和聚合列表，不恢复旧文件 |
| `usage.WithoutRefresh` | `credential.WithPolicy(..., ReadOnly)`，统一入口处理兼容参数 |
| 上游 Report / Window 序列化 | 复用字段语义，显式保留 fork 的 `ResetCredits` |
| 上游托盘直接读取旧 Provider 类型 | 使用 `[]provider.Provider`、Registry ID / 别名解析和统一 fetch |
| `replace fyne.io/systray => ./third_party/systray` | 内部包 `third_party/systray`；不保留相对 replace，避免破坏 `go install ...@latest` |
| systray 嵌套 module | 原始清单保存为 `upstream-go.mod.txt` / `upstream-go.sum.txt`，源码纳入主 module，补齐 `tray` 和平台约束 |

### 对外接口与兼容注意

- 窗口增加 `id`、已知时的 `duration_seconds` / `model`；Cursor 当前账期的 ID 为 `billing`。沿用上游 Provider ID 映射；这些 ID 不应从可变的展示标签反推。
- 可识别的账户仅输出 `account.id` / `account.email`。Codex ID token 只用于提取本地登录的 email 元数据，不作为认证验证；令牌不进入报告。
- JSON 空窗口为 `[]`，未知 `resets_at` 省略。JSON / YAML 中的 `used_percent` 仍是已用比例，完整 `reset_credits` 保留。
- `target_percent = 10 + 80 × Pace`，起点 10、终点 90；缺少周期或重置时间省略。上游最初的 0–100 线性 target 已被最新实现替代，不应再引入。
- 退出码为成功 `0`、未配置／未登录 `3`、其他失败 `1`。只读下令牌过期属于失败，不冒充“未配置”。聚合仍通过各 Provider 的结果／错误表达局部失败。
- Grok 的用户信息、Z.ai 的现金余额仍为 best-effort。批量和结构化输出约 15 秒后返回已有结果并取消迟到工作；不能让一个无响应 Provider 阻止全部输出。
- Muse 的无活动窗口不是 0% 用量。文件登录路径按登录令牌区分五分钟缓存；无内联令牌时委托 `muse serve`，不恢复旧 Keychain 读取或权限提示方案。

## 分类提交及上游映射

下面的提交均在本地当前分支。最终文档提交可通过 `git log --oneline -- docs/fork-merge-guide.md` 定位，避免在文档内自引用尚不存在的 SHA。

| 类别 | fork 提交 | 内容 |
| --- | --- | --- |
| 1 | `6d50715` | `chore(repo): migrate fork identity to iugamlabs` |
| 2 | `b4312d8` | `fix(providers): port upstream CLI compatibility fixes` |
| 3 | `806605c` | `fix(credential): unify no-refresh behavior and Kimi policies` |
| 4 | `5155a72` | `feat(usage): extend structured reports and exit statuses` |
| 5 | `ff60ee2` | `perf(providers): parallelize requests and bound batch waits` |
| 6 | `4bf5573` | `feat(muse): add subscription quota provider` |
| 7 | `b780cae` | `feat(tray): add optional tray app using fork architecture` |
| 8 | `d04f927` | `build(tray): add isolated desktop build and release targets` |

每个适配提交携带对应测试。补充的原子写入失败、批量 JSON 超时、缓存切换账户测试已分别归入凭据、性能、Muse 提交。

| 上游提交 | 变化 | fork 提交／处理结果 |
| --- | --- | --- |
| [`174550c`](https://github.com/McKean/aiquokka/commit/174550c) | Antigravity 官方 CLI 查询 | `b4312d8`；使用最终非交互 JSON 接口 |
| [`9e93191`](https://github.com/McKean/aiquokka/commit/9e93191) | Kiro 启动、日期和进程清理 | `b4312d8`；保留 fork 官方 CLI 凭据归属 |
| [`a6b71fd`](https://github.com/McKean/aiquokka/commit/a6b71fd) | 桌面托盘及 systray 补丁 | `b780cae`、`d04f927`；Registry 适配、剩余语义、独立构建及带许可证压缩包 |
| [`5ce75a6`](https://github.com/McKean/aiquokka/commit/5ce75a6) | no-refresh、结构化接口和退出码 | `806605c`、`5155a72`、`b780cae`；接入现有策略，并在统一 fetch 校验 Capabilities；DeepSeek 保持事实式余额 |
| [`a7c30a9`](https://github.com/McKean/aiquokka/commit/a7c30a9) | 并行请求与批量等待上限 | `ff60ee2`；保留 Grok 文件锁，加入取消和完成结果保护 |
| [`27281d6`](https://github.com/McKean/aiquokka/commit/27281d6) | Muse Provider | `4bf5573`；移植后续修正后的最终实现 |
| [`f19af2e`](https://github.com/McKean/aiquokka/commit/f19af2e) | Muse Keychain 读取诊断 | `4bf5573`；历史实现不直接移植；被 43653c7 的 CLI 方案取代 |
| [`cf38031`](https://github.com/McKean/aiquokka/commit/cf38031) | Keychain 单行诊断 | `4bf5573`；已被最终 CLI 方案取代 |
| [`bd1c86c`](https://github.com/McKean/aiquokka/commit/bd1c86c) | Keychain 授权命令提示 | `4bf5573`；已被最终 CLI 方案取代，不提示修改 Keychain 权限 |
| [`43653c7`](https://github.com/McKean/aiquokka/commit/43653c7) | Muse CLI JSON-RPC 回退 | `4bf5573`；最终方案；替代上述三个 Keychain 诊断提交 |
| [`c2783dc`](https://github.com/McKean/aiquokka/commit/c2783dc) | Antigravity 窗口标签 | `b4312d8`；按 Weekly / 5h 命名，结构化数据仍为已用比例 |
| [`e7fea40`](https://github.com/McKean/aiquokka/commit/e7fea40) | Claude 配置目录及账户文件 | `b4312d8`、`5155a72`；读取配置目录并写回对应来源 |
| [`aee7203`](https://github.com/McKean/aiquokka/commit/aee7203) | 结构化 target 比例 | `5155a72`；使用 66ec5c1 修正后的 10–90 目标 |
| [`57ff2de`](https://github.com/McKean/aiquokka/commit/57ff2de) | target 不进入渲染器 | `5155a72`；终端和托盘标记继续使用原始 Pace |
| [`be1c8c9`](https://github.com/McKean/aiquokka/commit/be1c8c9) | Muse 慢启动超时 | `4bf5573`；15 秒默认值、可配置和 stderr 诊断 |
| [`f6e66b9`](https://github.com/McKean/aiquokka/commit/f6e66b9) | 超时经过时间及测试隔离 | `4bf5573`；保留上游修正 |
| [`e8c59ab`](https://github.com/McKean/aiquokka/commit/e8c59ab) | 子进程组测试启动同步 | `4bf5573`；使用同步后的假 CLI 测试 |
| [`e4c8430`](https://github.com/McKean/aiquokka/commit/e4c8430) | Muse 空报告的文件登录提示 | `4bf5573`；保留 logout 在先的 TBH file 登录说明 |
| [`430ceeb`](https://github.com/McKean/aiquokka/commit/430ceeb) | 空 windows 为数组 | `5155a72`；保留 Extra 和 ResetCredits 的 JSON 序列化 |
| [`a0974df`](https://github.com/McKean/aiquokka/commit/a0974df) | 窗口 model 字段 | `5155a72`；保留 Weekly Fable 模型信息 |
| [`87b658c`](https://github.com/McKean/aiquokka/commit/87b658c) | Z.ai 资源包 model 字段 | `5155a72`；保留资源包／现金窗口差异 |
| [`66ec5c1`](https://github.com/McKean/aiquokka/commit/66ec5c1) | target 缓冲语义 | `5155a72`；最终 10–90 目标，上游同步终点 |

## 后续同步步骤

1. 确保工作区干净，记录当前 fork HEAD 与本指南中的上游同步 SHA。检查 remote 是否仍对应正确仓库；仅在尚未配置时添加 `upstream`。

   ```sh
   git status --short --branch
   git remote -v
   git fetch --no-tags upstream main
   last_upstream=66ec5c129bdaba55d5ee2b14f1e8c2b54fe3c4cd
   git log --reverse --oneline "$last_upstream"..upstream/main
   git diff --stat "$last_upstream" upstream/main
   ```

2. 固定本轮目标 SHA，再审阅它相对上次同步点的代码差异、最终文档和测试。若存在修正后又替代的实现，按最终状态移植，并逐条记录处理理由。
3. 先迁移 Provider 兼容修复、凭据和数据模型，再迁移性能、新 Provider、UI／构建。保留现有目录、单一 Registry 和 fork 特有字段；不要直接用上游整份 `cmd/root.go`、`cmd/run.go`、数据模型或 `go.mod` 覆盖本地文件。
4. 每类功能和相应测试一起提交，正文添加一行或多行 `Upstream-Commit: <完整 SHA>`。提交前检查 `git diff --cached --check` 和 staged diff，避免夹带临时文件、真实凭据及构建产物。
5. 完成下述验证后，更新本指南的同步终点、提交映射、兼容差异和验证结果。先完成全部分类提交，再创建最后的文档提交。
6. 推送、打标签和发布按另一次明确授权执行；分类移植不使用伪造 merge 祖先的方式标记同步。

## 构建、验证与发布边界

### 默认 CLI 与独立托盘

```sh
# 默认 CLI：六个平台均可关闭 CGO 交叉编译
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o temp/aiquokka-macos-arm64 .

# Linux 托盘
CGO_ENABLED=0 go build -tags tray -o temp/aiquokka-tray .
# macOS 托盘：在具备 macOS SDK 的环境中执行
CGO_ENABLED=1 go build -tags tray -o temp/aiquokka-tray .

# 单独打包二进制、许可证及补丁说明，默认写入 dist/tray
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 scripts/build-tray.sh
# 设置版本时可使用 AIQUOKKA_VERSION=vX.Y.Z；开发构建默认 dev
```

托盘包中的二进制名为 `aiquokka-tray-{linux,macos}-{amd64,arm64}`，运行时仍需传入 `tray`（或 `bar` / `menu` / `systray`）。默认 CLI 的同名命令提供可选构建提示。Windows 本轮继续提供 CLI，不把托盘静默纳入默认构建。

macOS 使用系统字体与原生偏好窗口；Linux 使用 D-Bus 菜单。托盘显示剩余，默认阈值 80% 指的是已用比例。`-p` / `--pin` 解析 Registry 的 ID、别名及显示名；设置保存在用户配置目录的 `aiquokka/tray.json`，显式参数优先。不支持结构化输出或 watch 参数。

第三方源码、原始清单、Apache-2.0 / MIT 许可证与修改清单保存在 `third_party/`。内部包的 build tags 和 import 路径也是 fork 补丁，更新依赖时必须重新应用，特别注意 `.m` 文件的构建隔离。分发托盘包必须包含许可证及第三方说明。

### 验证命令和覆盖点

```sh
mkdir -p temp/go-cache temp/go-work
export GOCACHE="$PWD/temp/go-cache"
export GOTMPDIR="$PWD/temp/go-work"
go mod tidy
git diff --exit-code -- go.mod go.sum
go test ./...
go test -race ./...
go test -race -tags tray ./...
go vet ./...
go vet -tags tray ./...
go build ./...
```

完整测试需要允许 loopback HTTP 服务和假 CLI 子进程的环境；遇到沙箱权限错误应换到允许这些测试的环境复验，不应跳过测试并宣称通过。测试仅使用临时凭据、模拟 HTTP 和假 CLI，不验证真实账户或执行真实登录。

本次已验证：

- 默认 CLI 全量测试；托盘全量测试及 race 检查，涵盖内部 systray 包；凭据、Provider 并行查询、Muse 和命令层 race 检查。
- Kimi 过期／401 只读不写、原子写入失败后原凭据不变、轮换令牌能力校验；Grok 两个请求同时被拒绝只刷新一次。
- Codex 完整重置明细、JSON / YAML 新字段、空窗口、未知时间、target 边界、账户字段及退出码；保留 Cursor 权威用量和事实式余额。
- 慢 Provider 与取消后的批量／JSON 输出、Kiro / Muse 进程清理、Muse 跨登录缓存隔离、无活动窗口和超时诊断。
- 托盘剩余条／节奏、金额与重置事实、通知去重和重置误报、稳定 ID、关闭菜单通道、配置优先级与 Registry 别名。
- Windows / macOS / Linux 的 amd64 / arm64 六个默认 CLI 构建（CGO=0），Linux amd64 / arm64 托盘构建。
- 默认和托盘 `go vet`、模块一致性、工作流 YAML 与 actionlint、打包脚本语法、Linux 托盘压缩包的版本注入及许可证内容。

本次未在本机验证：macOS 原生托盘编译／运行、真实 macOS 或 Linux 桌面交互、桌面通知及真实 Provider 在线接口。macOS 原生测试与编译已配置到 CI，但此次没有推送，因此尚无远程 CI 运行结果。

CI 分别验证默认与 tray 构建，并检查默认依赖图不包含桌面实现。发布工作流只在 `v*` 标签触发：先构建四个托盘包，再由 GoReleaser 构建六个默认 CLI，最后上传托盘包和单独的 `TRAY_SHA256SUMS`；默认 CLI 继续使用 `SHA256SUMS`。所有任务来自同一标签提交。本次只修改配置，没有触发发布。

## 回退

可逐类 `git revert <fork 分类提交>`；回退涉及后续依赖时按本次提交顺序反向处理，整轮回退还需包括最终文档提交。回退 Registry / 数据模型时同时回退依赖它们的 Muse、托盘及对应测试。回退后重新运行默认与托盘验证，并将同步记录标为已回退，避免把不再存在的功能继续计入已处理范围。

仓库 remote 配置不属于 Git 提交，revert 不会恢复旧地址；本 fork 的正式身份仍是 iugamlabs。已存在的用户凭据和托盘设置不由代码回退自动删除。
