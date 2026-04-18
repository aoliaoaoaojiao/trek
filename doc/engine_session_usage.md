# 单线程引擎入口说明

`pkg/engine` 提供了面向调用方的稳定入口，适合单线程工具直接接入。

## 基本用法

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

## 运行时配置

可通过 `session.LoadConfigFile(path)` 加载 JSON 配置，支持以下字段（`LoadPreferenceFile` 仍保留为兼容别名，不建议新代码使用）：

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

## 说明

- `custom_events.actions` 支持通过 `xpath`、`resourceID`、`contentDescription`、`text`、`classname` 或 `bounds` 定位目标控件。
- 当只提供定位条件且未显式指定 `action` 时，默认按 `CLICK` 处理。
- `CheckPointInBlackRects` 可直接复用已加载的 `black_rects` 配置。
