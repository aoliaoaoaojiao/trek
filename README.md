# Trek

Trek 是一个面向 Android 真机的 UI 自动化遍历引擎。

它围绕“感知 -> 决策 -> 执行”闭环工作：抓取页面信息，生成候选动作，执行点击/返回/滚动，并在卡住时结合恢复策略、记忆层和可选的 OCR / 页面控件检测能力继续推进遍历。

## 能力概览

- 支持 `Smart Monkey` 式自主遍历
- 支持 `Reuse`、`UCTBandit` 等决策算法
- 支持 Goja 插件扩展页面转换、动作拦截和步骤回调
- 支持阻塞检测、恢复规划、经验记忆
- 支持截图输入、OCR 候选补充、基于 LLM 的页面控件检测
- 提供 CLI 运行模式和 Web 配置界面

## 目录结构

```text
cmd/                    CLI 与 Web 入口
pkg/monkey/             遍历编排与执行
pkg/coordinator/        稳定决策协调入口
internal/engine/        决策、恢复、候选、记忆、运行时
internal/engine/perception/providers/     Provider 对外入口与 OCR 等非 LLM provider
internal/engine/perception/providers/llm/ LLM 页面控件检测相关实现
internal/engine/perception/pagecontrol/   页面控件检测公共提示词、schema 与解析逻辑
docs/                   设计与实现文档
```

## 快速开始

### 1. 准备环境

- Go `1.23.6` 或更高版本
- Android 设备或模拟器
- `adb` 可用

如果 `adb` 不在系统 `PATH` 中，可以通过环境变量指定：

- `ADB_PATH`
- `ANDROID_HOME`
- `ANDROID_SDK_ROOT`

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置环境变量

Trek 启动时会自动加载项目根目录下的：

- `.env`
- `.env.development.local`
- `.env.local`

推荐将密钥写入 `.env.local`，不要直接写进代码。

示例：

```env
# OCR paddleocr
PADDLEOCR_API_URL=your_paddleocr_api_url
PADDLEOCR_API_KEY=your_aistudio_token

# 恢复经验库（可选）
RECOVERY_MEMORY_FILE=.\data\recovery.sqlite

# 页面控件检测 LLM（HTTP Provider）
LLM_API_URL=https://your-llm-gateway/v1/chat/completions
LLM_API_KEY=your_page_control_llm_key
LLM_MODEL=your-model-name

# OpenAI Chat Completions 兼容 Provider（页面控件检测，可选）
OPENAI_API_URL=https://api.openai.com/v1/chat/completions
OPENAI_API_KEY=your_openai_api_key
OPENAI_MODEL=gpt-4.1-mini

# Anthropic Messages Provider（页面控件检测，可选）
ANTHROPIC_API_URL=https://api.xiaomimimo.com/anthropic
ANTHROPIC_API_KEY=your_anthropic_api_key
ANTHROPIC_MODEL=mimo-v2.5

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

如果希望在运行结束后落盘一份报告：

```bash
trek run --package com.example.app --capture-screenshot --report-file .\log\run-report.json
```

也支持直接输出 Markdown 复盘文档：

```bash
trek run --package com.example.app --report-file .\log\run-report.md
```

说明：

- `--report-file`：指定报告输出路径
- `--report-format`：可选，支持 `json`、`md`
- `--artifact-dir`：可选，指定原始截图/XML 产物目录
- 未显式指定 `--report-format` 时，会按 `--report-file` 的扩展名自动推断
- 当设置了 `--report-file` 且未指定 `--artifact-dir` 时，会自动生成同名目录，例如 `run-report_artifacts`
- `.json` 会输出完整结构化报告，适合脚本消费
- `.md` 会输出人工可读的复盘摘要，适合直接查看
- 原始截图和 XML 会按页面名分目录输出，并以 `step-步骤号-before/after-动作名` 命名，便于复盘同页多次访问

JS 配置可以为项目提供默认运行开关，例如：

```js
const config = {
  capture_screenshot: true,
  keep_step_records: false,
}
```

这两个参数的优先级为：`CLI > JS 配置 > 默认值`。

同样支持配置“页面控件信息获取策略”：

```js
const config = {
  // raw: 直接使用 dump 原始 XML
  // ocr: 基于截图 OCR 提取控件区域并生成伪控件树
  // llm: 基于截图 LLM 推断控件区域并生成伪控件树
  page_control_strategy: "ocr", // 页面理解策略
  page_control_cache_file: "./data/page_control_cache.sqlite", // 可选：页面理解持久化缓存
  page_control_cache_ttl_seconds: 1800, // 可选：缓存 TTL，默认 1800 秒
}
```

当“页面理解策略” `page_control_strategy` 为 `ocr` 或 `llm` 时，Trek 会自动启用截图采集；如果当前步骤拿不到 dump，会继续尝试走“截图 -> 控件区域 -> 伪 XML”链路，而不是直接中断该步。

如果希望跨多次跑测复用页面理解结果，可以额外开启页面理解持久化缓存：

- JS 配置：`page_control_cache_file`
- 环境变量：`PAGE_CONTROL_CACHE_FILE`

启用后，`ocr` / `llm` 生成的合成 XML 会按截图指纹持久化到本地 SQLite；后续遇到相同页面截图时，会优先复用缓存结果，减少重复 OCR / LLM 调用。

当前推荐的缓存刷新逻辑为：

- 缓存 TTL 到期后自动重新获取
- 动作执行失败时，当前页面截图对应的缓存会立即失效
- 检测到 `same_page_no_change` / `high_visit_low_reward` 这类低收益信号时，当前页面截图对应的缓存也会失效
- 阻塞恢复链路中本来就会强制刷新，不复用旧缓存

其中 `llm` 现已使用专门的“控件检测 schema”，要求模型直接返回控件区域列表（`controls`），不再复用恢复动作建议的 `candidates` schema。页面控件提示词独立存放在 Markdown 文档中，并通过 Go `embed` 加载，便于单独维护。控件输出以基础交互类型 `action_type` 为主，例如 `click`、`drag`、`swipe_*`、`input`；可选的 `control_type` 仅作为语义补充。控件 `bounds` 优先使用对象格式 `{left,top,right,bottom}`，同时兼容四元数组 `[left, top, right, bottom]`。

如果页面名策略使用 `image_fingerprint`，还可以额外指定一个或多个局部指纹区域（坐标范围均为 `0~1`）：

```js
const config = {
  page_name_strategy: "image_fingerprint",
  image_similarity_ssim_threshold: 0.985,
  image_fingerprint_regions: [
    { left: 0.12, top: 0.22, right: 0.88, bottom: 0.78 },
  ],
}
```

其中：

- `image_fingerprint_regions`：指定需要重点比较的局部区域
- `image_similarity_ssim_threshold`：控制截图二次确认的灵敏度，越接近 `1` 越严格

这些配置比较适合滚动列表、对话区、卡片流等“局部内容变化明显、整体结构较稳定”的页面。

同样支持在 JS 中配置恢复相关调参（示例）：

```js
const config = {
  recovery_cooldown_steps: 2,
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

页面控件检测链路支持两种方式：

- 通用 HTTP Provider
- OpenAI Chat Completions 兼容 Provider
- Anthropic Messages Provider

常用环境变量：

- `RECOVERY_MEMORY_FILE`
- `LLM_API_URL`
- `LLM_API_KEY`
- `LLM_MODEL`
- `OPENAI_API_URL`
- `OPENAI_API_KEY`
- `OPENAI_MODEL`
- `ANTHROPIC_API_URL`
- `ANTHROPIC_API_KEY`
- `ANTHROPIC_MODEL`

使用建议：

- 恢复经验库可通过 `RECOVERY_MEMORY_FILE` 指定本地持久化路径
- 通用 HTTP Provider：至少配置 `LLM_API_URL`、`LLM_MODEL` 和 `LLM_API_KEY`
- OpenAI Chat Completions 兼容 Provider：至少配置 `OPENAI_MODEL` 和 `OPENAI_API_KEY`
- Anthropic Messages Provider：至少配置 `ANTHROPIC_MODEL` 和 `ANTHROPIC_API_KEY`
- 如果使用 OpenAI 兼容网关或代理，可额外配置 `OPENAI_API_URL`
- 如果使用 Anthropic 兼容网关或米莫 Anthropic 接口，可额外配置 `ANTHROPIC_API_URL`
- 当前带截图的页面控件检测属于多模态请求；以米莫兼容接口为例，只有 `mimo-v2.5` 与 `mimo-v2-omni` 支持图片输入，更适合这条链路
- 这些外部服务配置统一走环境变量，不再通过 `trek run` 传入
- 当前内置 LLM 仅用于 `page_control_strategy=llm` 的页面理解/控件检测，不再直接参与恢复决策或候选增强

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
- 协调入口：[coordinator.go](/h:/CodeProject/GoProject/trek-dev/pkg/coordinator/coordinator.go)
- LLM Provider 入口：[llm.go](/h:/CodeProject/GoProject/Trek/internal/engine/perception/providers/llm.go)
- OCR Provider：[ocr_http.go](/h:/CodeProject/GoProject/Trek/internal/engine/perception/providers/ocr_http.go)

## 开发说明

如果你正在做遍历框架、恢复策略或候选增强相关开发，建议同时参考 `docs/` 下的设计文档，尤其是：

- `docs/superpowers/specs/2026-04-26-agent-traversal-design.md`
