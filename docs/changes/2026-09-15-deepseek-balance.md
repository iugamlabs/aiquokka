# Plan: 优化 DeepSeek 余额展示

## 背景

DeepSeek 与 Codex / Grok / Cursor 不同，它没有周期性 quota，也没有总额度，只提供当前账户余额：

- `total_balance`
- `granted_balance`
- `topped_up_balance`

当前 aiquokka 会把 DeepSeek 的余额通过 `remainingBar()` 显示成满格绿色进度条：

```text
Balance [████████████████████████] ¥3.98
```

这个视觉容易误导用户理解为“剩余 100%”，实际上 DeepSeek 并不存在可计算的百分比。

## 目标

将 DeepSeek 这类“预付费余额型 Provider”与“Quota 型 Provider”在 UI 上区分开。

DeepSeek 改为纯金额展示，不再显示 progress bar。

目标效果：

```text
DeepSeek
────────
  Balance:       ¥3.98
  Granted:       ¥0.00
  Topped up:     ¥3.98
```

优先保持和现有 CLI 输出风格一致，不要额外增加复杂 UI。

## 修改建议

### 1. 不要把余额包装成普通 usage window

目前 `internal/providers/deepseek/deepseek.go` 中：

```go
report.Windows = append(report.Windows, usage.Window{
    Label:     "Balance" + tag,
    Remaining: &total,
    Currency:  info.Currency,
})
```

建议改成专门的金额 Fact，或者为 `usage.Report` 增加轻量的 Balance / Money 类型。

如果不想扩大模型改动，最简单方案是：

```go
report.Extra = append(report.Extra,
    usage.Fact{
        Label: "Balance" + tag,
        Value: usage.FormatMoney(total, info.Currency),
    },
)
```

然后继续输出：

```go
Granted
Topped up
```

这样无需使用 `Window.Remaining`。

### 2. 删除 DeepSeek 对 `remainingBar()` 的依赖

`internal/usage/render.go` 中的：

```go
remainingBar(...)
```

如果确认当前只有 DeepSeek 使用，可以直接删除。

如果其他 Provider 仍在使用，则保留，但 DeepSeek 不再走该逻辑。

不要影响以下 quota 型展示：

* Codex
* Grok
* Cursor
* 其他具有 percentage / used / limit / reset 的 Provider

### 3. 保留余额字段语义

DeepSeek API 字段保持原样：

* `Balance` = `total_balance`
* `Granted` = `granted_balance`
* `Topped up` = `topped_up_balance`

不要自行计算百分比。

不要尝试根据当前余额推算“剩余比例”。

### 4. 测试

更新 / 增加测试，至少覆盖：

* DeepSeek 有余额时不出现 `[████...]`
* `Balance ¥3.98`
* `Granted ¥0.00`
* `Topped up ¥3.98`
* 多币种情况下仍保留 `(CNY)` / `(USD)` 标签
* 余额为 0 时正常显示 `¥0.00`
* 不影响其他 Provider 的进度条渲染

## 设计原则

明确区分两种 Provider：

### Quota 型

例如：

```text
Codex  (plus)
─────────────
  5h       [███▓█████░░░░░░░░░░░░░░░]  37.0% left   resets in 38m (Tue 22:35)
  Weekly   [██████████████▓███░░░░░░]  76.0% left   resets in 4d2h (Sun 00:29)
  Resets:        3 available · 0 applicable now
    #1             expires in 5d9h (Mon 07:49)
    #2             expires in 18d11h (Sun 09:47)
    #3             expires in 19d9h (Mon 07:36)
```

适合使用：

* progress bar
* percentage
* reset time
* pace marker

### Balance 型

例如 DeepSeek：

```text
DeepSeek
────────
  Balance:       ¥3.98
  Granted:       ¥0.00
  Topped up:     ¥3.98
```

只显示真实金额，不使用进度条。

## 范围控制

本次只优化余额展示。

不要重构整个 `usage.Report`，除非确实有必要。
优先采用最小改动、低风险方案。



```
我倾向让 Codex **直接把 DeepSeek 的 `Balance` 也改成 `Fact`**，这是目前最干净的最小改动；暂时没必要为了一个 DeepSeek 专门引入新的 `Balance` 数据模型。等以后 OpenRouter、Anthropic prepaid、其他按余额计费 Provider 多起来，再统一抽象成 `MoneyBalance` 会更合适。 
```
