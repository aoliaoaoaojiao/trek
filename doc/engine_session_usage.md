# 单线程引擎入口使用说明

`pkg/session` 提供面向调用方的稳定会话入口，适合单线程工具直接接入。

## 基础用法（XML 输入）

```go
session := engine.NewSession(engine.Config{
    PackageName: "com.demo.app",
    Algorithm:   types.Reuse,
    DeviceType:  types.Phone,
})

operate, err := session.NextAction("LoginActivity", xmlContent)
if err != nil {
    return err
}

jsonText, err := session.NextActionJSON("LoginActivity", xmlContent)
if err != nil {
    return err
}
_ = operate
_ = jsonText
```

## 扩展输入（XML + 截图）

从当前版本开始，`Session` 支持扩展输入结构 `ActionInput`，用于为后续图像执行预留通道。

```go
input := engine.ActionInput{
    XMLDescOfGuiTree: xmlContent,
    Screenshot:       screenshotBytes, // 可选
}

operate, err := session.NextActionWithInput("LoginActivity", input)
if err != nil {
    return err
}

jsonText, err := session.NextActionJSONWithInput("LoginActivity", input)
if err != nil {
    return err
}
_ = operate
_ = jsonText
```

约束说明：
- `pageName` 不能为空。
- `XMLDescOfGuiTree` 与 `Screenshot` 不能同时为空。

## 感知模式切换

运行时支持三种感知模式：
- `xml-only`：只走 XML 感知（默认模式，兼容历史行为）
- `image-only`：只走图像感知（当前为占位实现，要求截图不为空）
- `hybrid`：优先 XML，失败后回退图像

```go
if err := session.SetObservationMode("hybrid"); err != nil {
    return err
}
currentMode := session.GetObservationMode()
_ = currentMode
```

## 运行时配置

可通过 `session.LoadConfigFile(path)` 加载 Goja 脚本配置（仅支持 `.js`）。
`LoadPreferenceFile` 仅保留为兼容别名，不建议新代码使用。
可在配置脚本中引用类型声明 [trek-plugin.d.ts](H:\CodeProject\GoProject\trek-dev\assets\goja\trek-plugin.d.ts)，获得配置项自动补全与类型提示。

```javascript
/// <reference path="./trek-plugin.d.ts" />
```

脚本可以同时暴露两个对象：
- `config`：静态配置入口，用于资源映射、黑名单区域等稳定配置。
- `plugin`：运行时策略插件入口，用于页面改造、决策前后动作干预、步骤结果观测。

```javascript
const config = {
  res_mapping: {
    login_button_alias: "com.demo:id/login",
  },
  black_rects: {
    LoginActivity: [[0, 0, 100, 100]],
  },
}

const plugin = {
  transformPage(ctx) {
    // 返回新的页面信息；不修改真实设备界面，只影响本轮引擎理解到的页面。
    return {
      name: ctx.page.name,
      xml: trek.page.patchText(ctx.page.xml, "old_button", "new_button"),
    }
  },

  beforeDecide(ctx) {
    const login = trek.page.findByText(ctx.page, "登录")
    if (!login) {
      return null
    }
    return trek.action.click(login.bounds, 300)
  },

  afterDecide(ctx, action) {
    // 返回 null 表示沿用引擎原始动作；返回 action 可替换本轮动作。
    return null
  },

  onStepResult(ctx) {
    if (ctx.result.crash || ctx.result.anr) {
      trek.state.set("last_fatal_page", ctx.result.before.name)
    }
    if (ctx.result.screenshot) {
      trek.log.info("本轮截图大小", ctx.result.screenshot.bytes.length)
    }
  },
}
```

如需单独获取改造后的页面信息，可调用：

```go
info, err := session.TransformPageInfo(pageName, xmlContent)
if err != nil {
    return err
}
_ = info.PageName
_ = info.XML
```

## 注意事项

- `plugin.transformPage` 用于返回新的页面信息，适合修正页面名、清洗 XML 或为策略补充可识别节点。
- `plugin.beforeDecide` 可在模型决策前直接返回动作；返回 `null` 或不实现时继续走默认引擎。
- `plugin.afterDecide` 可观察或替换默认引擎动作；返回 `null` 表示沿用原动作。
- `plugin.onStepResult` 会收到 crash/anr、截图、执行前后 XML 与页面名，适合做统计、状态记录和自定义恢复策略。
- 截图以 `Uint8Array` 形式暴露在 `ctx.page.screenshot.bytes` 或 `ctx.result.screenshot.bytes`。
- `CheckPointInBlackRects` 会复用已加载配置中的 `black_rects`。
- `config.res_mapping`、`config.black_rects`、`config.skip_all_actions_from_model` 作为静态配置保留，复杂运行时行为建议放入 `plugin`。

