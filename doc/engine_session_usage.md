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

可通过 `session.LoadConfigFile(path)` 加载 JSON 配置。
`LoadPreferenceFile` 仅保留为兼容别名，不建议新代码使用。

```json
{
  "res_mapping": {
    "login_button_alias": "com.demo:id/login"
  },
  "black_rects": {
    "LoginActivity": [[0, 0, 100, 100]]
  },
  "input_texts": ["demo_user"],
  "fuzzing_texts": ["fuzz_value"],
  "random_input_text": true,
  "do_input_fuzzing": false,
  "skip_all_actions_from_model": false,
  "custom_events": [
    {
      "prob": 1.0,
      "times": 1,
      "pageName": "LoginActivity",
      "actions": [
        {
          "action": "CLICK",
          "resourceID": "login_button_alias"
        }
      ]
    }
  ]
}
```

## 注意事项

- `custom_events.actions` 支持通过 `xpath`、`resourceID`、`contentDescription`、`text`、`classname` 或 `bounds` 定位目标控件。
- 当只提供定位条件且未显式指定 `action` 时，默认按 `CLICK` 处理。
- `CheckPointInBlackRects` 会复用已加载配置中的 `black_rects`。
- 当前分支中 `LoadResourceMapping` 的完整解析能力仍在建设中，配置字段是否生效需结合当前实现确认。

