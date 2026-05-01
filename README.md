# Trek

Trek 是一个面向 Android 真机的 UI 自动化遍历引擎。

它围绕“感知 -> 决策 -> 执行”闭环工作：抓取页面信息，生成候选动作，执行点击/返回/滚动，并在卡住时结合恢复策略、记忆层和可选的 OCR / LLM 能力继续推进遍历。

## 能力概览

- 支持 `Smart Monkey` 式自主遍历
- 支持 `Reuse`、`UCTBandit` 等决策算法
- 支持 Goja 插件扩展页面转换、动作拦截和步骤回调
- 支持阻塞检测、恢复规划、经验记忆
- 支持截图输入、OCR 候选增强、LLM 恢复候选
- 提供 CLI 运行模式和 Web 配置界面

## 目录结构

```text
cmd/                    CLI 与 Web 入口
pkg/monkey/             遍历编排与执行
pkg/session/            稳定决策会话入口
internal/engine/        决策、恢复、候选、记忆、运行时
docs/                   设计与实现文档
```

## 快速开始

### 1. 准备环境

- Go `1.23.6` 或更高版本
- Android 设备或模拟器
- `adb` 可用

如果 `adb` 不在系统 `PATH` 中，可以通过环境变量指定：

- `ADB_PATH`
- `TREK_ADB_HOME`
- `ANDROID_HOME`
- `ANDROID_SDK_ROOT`

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置环境变量

Trek 启动时会自动加载项目根目录下的：

- `.env`
- `.env.local`

推荐将密钥写入 `.env.local`，不要直接写进代码。

示例：

```env
# OCR paddleocr
PADDLEOCR_API_URL=your_paddleocr_api_url
PADDLEOCR_API_KEY=your_aistudio_token

# 恢复经验库（可选）
RECOVERY_MEMORY_FILE=.\data\recovery.sqlite

# 恢复链路 LLM（HTTP Provider）
LLM_API_URL=https://your-llm-gateway/v1/chat/completions
LLM_API_KEY=your_recovery_llm_key
LLM_MODEL=your-model-name

# OpenAI Responses Provider（可选）
OPENAI_API_URL=https://api.openai.com/v1/responses
OPENAI_API_KEY=your_openai_api_key
OPENAI_MODEL=gpt-4.1-mini

# ADB（可选）
# ADB_PATH=C:\Android\platform-tools\adb.exe
```

### 4. 运行遍历

```bash
trek run --package com.example.app --capture-screenshot
```

如果要自动使用当前前台应用：

```bash
trek run --auto-current-app --capture-screenshot
```

如果需要加载 JS 配置文件：

```bash
trek run --package com.example.app --config .\config.generated.js --capture-screenshot
```

JS 配置可以为项目提供默认运行开关，例如：

```js
const config = {
  capture_screenshot: true,
  keep_step_records: false,
}
```

这两个参数的优先级为：`CLI > JS 配置 > 默认值`。

同样支持在 JS 中配置恢复与候选调参（示例）：

```js
const config = {
  recovery_cooldown_steps: 2,
  llm_max_calls: 3,
  llm_window_steps: 30,
  recovery_two_state_loop_threshold: 2,
  recovery_high_visit_threshold: 8,
  recovery_low_reward_window: 6,
  candidate_ambiguity_top_gap_threshold: 0.15,
  high_value_page_visit_limit: 2,
  candidate_risk_drop_threshold: 2.1,
  candidate_min_fusion_score: -0.3,
}
```

推荐将“配置声明”和“行为插件”拆分，降低单文件复杂度：

```js
// config.js
const config = {
  page_source: "uia",
  plugins: [
    "./plugins/recovery.plugin.js",
    "./plugins/anti_loop.plugin.js",
  ],
}
```

`plugins` 会按数组顺序加载并执行。

仓库内可直接参考：

- `examples/config-split/config.js`
- `examples/config-split/plugins/normalize-page.plugin.js`
- `examples/config-split/plugins/recovery-guard.plugin.js`

运行示例：

```bash
trek run --package com.example.app --config .\examples\config-split\config.js
```

## 常用命令

运行 Monkey：

```bash
trek run --package com.example.app
```

探测当前页面名：

```bash
trek run --package com.example.app --probe-page-name
```

启动 Web 配置界面：

```bash
trek web
```

## OCR 配置

当前探索链路支持通过截图调用外部 OCR 服务，把识别出的文本区域直接映射为 `CLICK` 候选。

### Aistudio layout-parsing

当 `PADDLEOCR_API_URL` 包含 `/layout-parsing` 时，Trek 会自动切换为 Aistudio 请求格式：

- Header 使用 `Authorization: token <PADDLEOCR_API_KEY>`
- 请求体包含：
  - `file`
  - `fileType=1`
  - `useDocOrientationClassify=false`
  - `useDocUnwarping=false`
  - `useChartRecognition=false`

最简单的启动方式：

```bash
trek run --package com.example.app --capture-screenshot
```

只要 `.env` 中已经配置：

- `PADDLEOCR_API_URL`
- `PADDLEOCR_API_KEY`

就会自动启用 OCR 候选增强。

## LLM 配置

恢复链路支持两种方式：

- 通用 HTTP Provider
- OpenAI Responses Provider

常用环境变量：

- `RECOVERY_MEMORY_FILE`
- `LLM_API_URL`
- `LLM_API_KEY`
- `LLM_MODEL`
- `OPENAI_API_URL`
- `OPENAI_API_KEY`
- `OPENAI_MODEL`

使用建议：

- 恢复经验库可通过 `RECOVERY_MEMORY_FILE` 指定本地持久化路径
- 通用 HTTP Provider：至少配置 `LLM_API_URL`、`LLM_MODEL` 和 `LLM_API_KEY`
- OpenAI Responses Provider：至少配置 `OPENAI_MODEL` 和 `OPENAI_API_KEY`
- 如果使用 OpenAI 兼容网关或代理，可额外配置 `OPENAI_API_URL`
- 这些外部服务配置统一走环境变量，不再通过 `trek run` 传入

## 配置优先级

同一项配置同时存在时，推荐按下面顺序理解：

1. CLI 参数
2. 环境变量
3. 默认值

这意味着：

- 临时调试时优先用 CLI
- 日常本地开发优先用 `.env.local`
- CI / 服务器环境优先用平台 Secret 注入

## 安全建议

- 不要在代码里硬编码 API Key 或 Token
- 不要把 `.env.local` 提交到仓库
- 不要在日志中打印完整密钥
- 如果密钥曾暴露，及时轮换

## 相关代码位置

- CLI 入口：[root.go](/h:/CodeProject/GoProject/trek-dev/cmd/root.go)
- 运行命令：[run.go](/h:/CodeProject/GoProject/trek-dev/cmd/run.go)
- 会话装配：[session.go](/h:/CodeProject/GoProject/trek-dev/pkg/session/session.go)
- OCR Provider：[ocr_http.go](/h:/CodeProject/GoProject/trek-dev/internal/engine/candidate/providers/ocr_http.go)

## 开发说明

如果你正在做遍历框架、恢复策略或候选增强相关开发，建议同时参考 `docs/` 下的设计文档，尤其是：

- `docs/superpowers/specs/2026-04-26-agent-traversal-design.md`
