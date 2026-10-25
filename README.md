# aiquokka

在终端里统一查看 AI 编程助手的剩余额度、重置时间和 API 账户余额。

本项目是 [McKean/aiquokka](https://github.com/McKean/aiquokka) 的 fork，由 [iugamlabs](https://github.com/iugamlabs/aiquokka) 维护。复用官方 CLI / 客户端已有的登录凭据，也支持通过环境变量提供 API Key。

![aiquokka 终端界面](docs/screenshot.webp)

## 本 fork 的特色

- **Cursor 支持**：自动发现 Cursor Agent、系统凭据存储和 Cursor Desktop 的凭据，查看当前账期额度。使用服务端返回的使用率；缺少百分比时显示未知，不根据消费金额猜测额度。
- **直观的剩余额度**：进度条与 `left` 百分比统一表示剩余额度，配合绿 / 黄 / 红颜色和青色用量节奏标记；聚合视图中的进度条统一对齐。
- **凭据自动恢复**：默认 `auto` 策略在支持的 Provider 上静默刷新过期凭据并安全写回，保留 `readonly` 选项。Codex / Grok 在满足条件的单 Provider 交互查询中支持调用官方登录命令恢复认证。
- **Codex 重置次数详情**：区分剩余重置次数与当前可用次数，展示重置凭证的到期时间；终端最多列出三条到期明细，JSON / YAML 保留完整返回数据。
- **清晰的账户余额**：DeepSeek 直接展示余额、赠送余额和充值余额，不再用进度条表达金额；新增 OpenRouter 余额、累计额度与已消费金额查询。
- **Muse 与桌面托盘**：新增 Muse 订阅额度查询；可选托盘构建支持 macOS / Linux，在菜单栏查看剩余额度并接收告警。默认 CLI 继续保持纯 Go 跨平台构建。
- **统一 Provider 注册**：子命令、聚合查询与 `--list` 共用同一份 Provider 清单，便于扩展和维护。
- **跨平台分发**：提供 Windows、macOS、Linux 的 amd64 / arm64 构建，以及 Homebrew、Scoop 和 Go 安装方式。

同时保留多 Provider 并发查询、实时刷新和 JSON / YAML 输出。聚合查询会跳过未配置的服务，单个服务报错不影响其余结果；交互终端按固定顺序展示，并随着响应到达逐项更新。非交互聚合查询约等待 15 秒，保留已完成结果并为未响应服务输出错误。

## 安装

### Homebrew

```sh
brew tap iugamlabs/tap
brew install aiquokka
```

### Scoop

```powershell
scoop bucket add iugamlabs https://github.com/iugamlabs/scoop
scoop install aiquokka
```

### Go

需要 Go 1.26.5 或更高版本，具体要求见 [go.mod](go.mod)。

```sh
go install github.com/iugamlabs/aiquokka@latest
```

也可以从 [Releases](https://github.com/iugamlabs/aiquokka/releases) 下载对应平台的二进制，或在源码目录构建：

```sh
go build -o aiquokka .
```

## 使用

先在官方 CLI / 客户端登录，或为使用 API Key 的服务配置下表中的环境变量，然后运行：

```sh
aiquokka                  # 汇总所有已配置的服务
aiquokka --list           # 列出支持的 Provider，也可用 aiquokka list
aiquokka codex            # 仅查看 Codex
aiquokka cursor           # 仅查看 Cursor
aiquokka muse             # 仅查看 Muse
aiquokka --watch          # 每 60 秒刷新全部服务
aiquokka codex -w         # 持续查看单个服务
aiquokka --version        # 查看版本
aiquokka --help           # 查看帮助
```

Watch 模式下，交互终端会显示下次刷新的倒计时。按 `r` 立即刷新，按 `q` 或 Ctrl+C 退出。

### 支持的服务与凭据来源

| 命令 | 查询内容 | 凭据来源 |
| --- | --- | --- |
| `claude` | 5 小时、每周及 Weekly Fable 额度 | `$CLAUDE_CONFIG_DIR/.credentials.json`（默认 `~/.claude/.credentials.json`），或 macOS Keychain |
| `codex` | 5 小时 / 每周额度、重置次数与到期明细 | `~/.codex/auth.json` |
| `cursor` | 当前账期额度、Auto / API 使用率及按量消费信息 | Cursor Agent 登录文件、系统凭据存储，或 Desktop 的 `state.vscdb` |
| `kimi` | Kimi Code 的 5 小时及每周额度 | `~/.kimi-code` / `~/.kimi` 中的 OAuth 凭据，或 `KIMI_API_KEY` |
| `grok` | 周期额度、订阅等级与 Grok Code 状态 | `~/.grok/auth.json` |
| `copilot` | Premium、Chat、Completions 额度 | GitHub Copilot 配置目录中的 `apps.json` / `hosts.json` |
| `deepseek` | 余额、赠送余额与充值余额 | `DEEPSEEK_API_KEY` |
| `kiro` | 月度积分、套餐、重置日期与超额状态 | 调用已安装的 `kiro-cli /usage`，由官方 CLI 管理凭据 |
| `antigravity`（别名 `agy`） | Antigravity 配额 | 调用已登录的官方 `agy -p /usage --output-format json` |
| `zai` | 预付费资源包与现金余额 | `ZAI_API_KEY`，或 `~/.pi/agent/models.json` 中的 zai 配置 |
| `openrouter` | 账户余额、累计额度与已消费金额 | `OPENROUTER_MANAGEMENT_KEY` |
| `muse` | 5 小时及每周订阅额度 | `MUSE_AUTH_PATH` 或 `$XDG_CONFIG_HOME/muse/auth.json`（默认 `~/.config/muse/auth.json`）；缺少内联令牌时调用 `muse serve` |

各服务只展示接口实际返回的数据，具体窗口和字段取决于账户与套餐。直接指定未配置的 Provider 会显示配置提示。

API Key 配置示例（sh / bash / zsh）：

```sh
export DEEPSEEK_API_KEY="你的 DeepSeek API Key"
export OPENROUTER_MANAGEMENT_KEY="你的 OpenRouter Management API Key"
aiquokka
```

OpenRouter 必须使用 **Management API Key**，普通推理 API Key 无法查询账户额度。余额按接口返回的 `total_credits - total_usage` 计算，以美元显示。

Z.ai 中国平台可设置 `ZAI_BASE_URL=https://open.bigmodel.cn/api`，余额以人民币显示。资源包按模型适用范围分别展示，可通过 `Applies to` 字段查看。

### 如何阅读额度

- 实心 `█` 表示剩余额度，灰色 `░` 表示已用额度，`left` 是剩余百分比。
- 剩余超过 40% 为绿色，15%–40% 为黄色，低于 15% 为红色。
- 青色标记表示按当前周期匀速消耗时，理论上应剩余的位置。剩余条超过标记表示消耗较慢，短于标记表示消耗较快；需要已知周期信息才会显示。
- 重置时间同时显示相对倒计时和本地时间。
- DeepSeek 与 OpenRouter 直接显示金额，不推算剩余百分比。

Cursor 有额度百分比时，默认隐藏可能与该额度含义冲突的 Provider 提示。需要排查时可启用调试信息：

```sh
AIQUOKKA_DEBUG=1 aiquokka cursor
```

### JSON / YAML 输出

```sh
aiquokka --json
aiquokka codex --yaml
aiquokka cursor --yml       # --yaml 的别名
aiquokka --watch --json     # 每次刷新输出一个新的 JSON 文档
```

聚合输出以 Provider 名称的小写形式为键，未配置的服务省略，查询失败的服务包含 `error` 字段。JSON 与 YAML 输出选项互斥。

结构化输出中的 `used_percent` 仍表示**已用百分比**，与终端显示的 `left` 含义不同。Watch 模式会连续输出多个文档，读取脚本需要逐次处理。

窗口增加稳定的 `id`，已知周期以 `duration_seconds` 表示，特定模型额度可包含 `model`。已知周期与重置时间时，`target_percent` 从周期起点的 10 线性增长到终点的 90（`10 + 80 × Pace`），是带缓冲的已用目标；它与终端原始线性节奏标记不同。缺失信息不推测，未知 `resets_at` 省略，`windows` 为空时为 `[]`。

可识别登录账户时，`account` 仅包含可用的 `id` / `email`，不输出令牌。Codex 的完整 `reset_credits` 保留。退出码：成功 `0`，指定服务未配置／未登录 `3`，其他失败 `1`；聚合查询仍以各服务的 `error` 字段表达局部错误。

## 凭据策略与认证恢复

默认策略为 `auto`：凭据正常时直接读取；过期时使用 Provider 声明支持的恢复方式。Claude、Codex、Cursor、Grok、Kimi 支持刷新并持久化新凭据。刷新令牌可能轮换，因此刷新后需要安全写回，避免丢弃新令牌。

```sh
aiquokka codex                              # 默认 auto
aiquokka --credential-policy readonly       # aiquokka 不刷新或写回凭据
aiquokka --no-refresh                       # readonly 的兼容入口
aiquokka --credential-policy persist codex   # 显式使用刷新并写回策略
```

| 策略 | 行为 |
| --- | --- |
| `auto` | 默认；按 Provider 能力选择安全恢复方式，无法恢复时提示通过官方 CLI 登录 |
| `readonly` | aiquokka 不刷新或写回凭据；Kiro、Antigravity 和 Muse CLI 查询仍由官方客户端管理自己的凭据 |
| `memory` | 仅在进程内刷新，不写回；当前所有 Provider 均不支持，使用时会报错 |
| `persist` | 在需要时刷新并安全写回；目前仅 Claude、Codex、Cursor、Grok、Kimi 支持 |

策略对本次调用的所有 Provider 生效，不用于切换账号。不支持的策略会明确报错。`--no-refresh` 可与显式 `readonly` 同用，与显式 `auto` / `memory` / `persist` 同用时报参数冲突。

Muse 文件登录查询在进程内缓存五分钟；未报告活动窗口时显示未知及提示。CLI 默认最多等待 15 秒，可用 `AIQUOKKA_MUSE_CLI_TIMEOUT=30s` 调整，但调用方更短的超时仍生效。macOS 上 CLI 登录成功却没有用量时，可先执行 `muse logout`，再执行 `TBH_CREDENTIAL_BACKEND=file muse login`，并在后续使用 Muse 时保持该环境变量。

Codex / Grok 的静默刷新成功后，原查询最多重试一次。如果需要重新登录，只有在**单独查询该 Provider、输入输出均为交互终端、非 watch、非 JSON / YAML、非 CI** 时，才可能自动运行 `codex login` 或 `grok login`，成功后重新查询。聚合查询和脚本环境会显示登录提示。

macOS 上，Claude 优先读取包含 OAuth access token 的凭据文件，否则通过系统 `security` 工具读取 Keychain。通过 SSH 使用时，如钥匙串被锁定，可先执行 `security unlock-keychain`。

## 可选桌面托盘

默认安装的 CLI 包含 `tray` 命令提示。完整托盘版本单独构建，支持 macOS / Linux；它也保留普通 CLI 子命令。托盘百分比和额度条表示剩余（`left`），告警阈值按**已用比例**计算，默认 80%。

```sh
# Linux
CGO_ENABLED=0 go build -tags tray -o aiquokka-tray .
# macOS：需要 Xcode Command Line Tools
CGO_ENABLED=1 go build -tags tray -o aiquokka-tray .

./aiquokka-tray tray                  # 每分钟刷新，默认监控全部服务
./aiquokka-tray tray -p agy           # 支持 Registry 中的 Provider ID / 别名
./aiquokka-tray tray --pin claude     # 顶栏固定 Claude，菜单仍显示全部服务
./aiquokka-tray tray --interval 2m --threshold 90
./aiquokka-tray tray --no-refresh
```

`bar`、`menu`、`systray` 是 `tray` 的别名。设置保存在系统用户配置目录的 `aiquokka/tray.json`，显式参数优先。macOS 提供原生偏好窗口，Linux 提供子菜单；刷新间隔至少一分钟。托盘不支持 `--watch` 或 JSON / YAML 输出。

可运行 `scripts/build-tray.sh` 生成包含许可证的独立压缩包；发布工作流生成 `aiquokka-tray-{linux,macos}-{amd64,arm64}.tar.gz` 和 `TRAY_SHA256SUMS`。默认 CLI 仍提供 Windows、macOS、Linux 六种纯 Go 构建。

## 注意事项

部分查询依赖官方客户端使用的未公开接口或本地凭据格式，可能随服务更新而失效。建议使用内置的 60 秒 watch 间隔，避免叠加多个监控进程或高频轮询导致限流。

## 来源与许可

感谢上游 [McKean/aiquokka](https://github.com/McKean/aiquokka) 提供基础实现。本 fork 在此基础上持续改进 Provider 支持、额度展示与凭据恢复。

维护与后续上游同步请参阅 [fork 合并指南](docs/fork-merge-guide.md)。

项目采用 [MIT License](LICENSE)。托盘附带的 systray 和图标许可见 [第三方说明](third_party/NOTICE.md)。
