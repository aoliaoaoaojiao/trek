# 运行报告输出最佳实践

当 Trek 作为本地 AI 驱动遍历工具使用时，建议每次关键实验、问题复现或参数调优后都保留一份运行报告，避免只依赖终端滚动日志做回溯。

## 推荐做法

- 日常调试优先输出 `json` 报告，便于后续脚本统计、二次分析或接入本地可视化工具。
- 需要人工复盘、同步结论或沉淀排查记录时，同时输出 `md` 报告。
- 如果本次运行需要定位失败步骤，建议保持 `--keep-step-records=true`，否则报告里不会有详细 step 记录。
- 如果页面理解依赖 OCR / LLM / 截图指纹，建议同时开启截图链路，让报告更完整反映真实执行上下文。
- 建议同时保留原始截图/XML 产物，并按页面名分目录保存，避免不同页面或同页多轮访问的文件混在一起。

## 推荐命令

```bash
trek run --package com.example.app --capture-screenshot --report-file .\log\run-report.json
```

```bash
trek run --package com.example.app --capture-screenshot --report-file .\log\run-report.md
```

```bash
trek run --package com.example.app --capture-screenshot --report-file .\log\run-report.json --artifact-dir .\log\run-artifacts
```

## 复盘关注点

- `stop_reason` 是否符合预期，例如 `completed`、`timeout`、`max_consecutive_failures`
- `page_visit_count` 是否出现明显的高频循环页面
- `action_count` 是否存在异常高频的返回、滚动或空操作
- `recovery_cooldown_*` 与 `out_of_app_recoveries` 是否异常偏高
- 最近失败步骤是否集中在同一页面、同一动作类型或同一控件区域
- 页面产物目录下，是否存在某个页面持续重复出现相似截图或相同 XML 结构

## 结论沉淀建议

- 对稳定复现的问题，保留对应报告文件路径并在问题记录中附上摘要结论。
- 对新发现的循环页、黑名单区域、无效动作模式，及时回写到配置、插件或最佳实践文档中。
