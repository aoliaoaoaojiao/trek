# 屏蔽页面区域示例

本示例展示如何使用 `excluded_touch_areas` 配置来屏蔽不该触碰的页面区域。

## 使用场景

- 系统状态栏（顶部）
- 系统导航栏（底部）
- 广告位
- 悬浮按钮
- 弹窗关闭按钮
- 任何不应该被点击的区域

## 配置说明

```javascript
excluded_touch_areas: [
  {
    page_name: "页面名",  // 与遍历日志中 page= 输出一致
    bounds: [left, top, right, bottom]  // 像素坐标或归一化坐标
  }
]
```

### 参数说明

| 参数 | 类型 | 说明 |
|------|------|------|
| `page_name` | string | 页面名，可在遍历日志中查看 |
| `bounds` | [number, number, number, number] | 排除矩形 [left, top, right, bottom] |

### 坐标系自动判断

系统会自动判断坐标系：
- **归一化坐标**：如果 bounds 所有值都 < 1，则认为是归一化坐标（0~1）
- **像素坐标**：如果 bounds 任何值 >= 1，则认为是像素坐标

| 坐标系 | 示例 | 说明 |
|--------|------|------|
| 像素坐标 | `[0, 0, 1080, 100]` | 适合固定分辨率设备 |
| 归一化坐标 | `[0, 0, 1.0, 0.052]` | 适合不同分辨率设备，推荐使用 |

## 如何获取页面名和坐标

1. **获取页面名**：
   - 启动 trek 后，在日志中查找 `page=XMLPage:xxxxxx` 格式的输出
   - 或在 Web 配置界面刷新预览后查看"页面名"字段

2. **获取坐标**：
   - 在 Web 配置界面的"界面截图"区域点击截图
   - 底部会显示点击位置的坐标信息
   - 使用"绝对坐标(设备原始)"的值
   - 归一化坐标 = 像素坐标 / 屏幕分辨率

## 配置方式

### 方式1：直接在配置文件中添加

在配置文件中添加 `excluded_touch_areas` 字段：

```javascript
const config = {
  page_source: "uia",
  page_name_strategy: "structure_fingerprint",
  // ... 其他配置

  excluded_touch_areas: [
    // 像素坐标示例
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1080, 100] },

    // 归一化坐标示例（推荐）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1.0, 0.052] },
  ],
}
```

### 方式2：通过 Web 配置界面导出后手动添加

1. 在 Web 配置界面完成其他配置
2. 点击"导出配置"下载配置文件
3. 在下载的配置文件中手动添加 `excluded_touch_areas` 字段
4. 使用修改后的配置文件运行 trek

## 示例

### 像素坐标示例

```javascript
excluded_touch_areas: [
  // 状态栏（顶部 100px）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1080, 100] },
  // 导航栏（底部 100px）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 1820, 1080, 1920] },
]
```

### 归一化坐标示例（推荐）

```javascript
excluded_touch_areas: [
  // 状态栏（顶部约 5.2%）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1.0, 0.052] },
  // 导航栏（底部约 5.2%）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0.948, 1.0, 1.0] },
  // 广告位（底部 10%~20% 区域）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0.8, 1.0, 0.9] },
  // 悬浮按钮（右下角）
  { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0.83, 0.78, 1.0, 0.89] },
]
```

### 为不同页面配置不同区域

```javascript
excluded_touch_areas: [
  // 首页的广告位
  { page_name: "XMLPage:home123", bounds: [0, 0.8, 1.0, 0.9] },
  // 详情页的分享按钮
  { page_name: "XMLPage:detail456", bounds: [0.83, 0, 1.0, 0.05] },
]
```

## 注意事项

1. **坐标系自动判断**：系统会自动判断是像素坐标还是归一化坐标
2. **推荐使用归一化坐标**：方便在不同分辨率设备间复用配置
3. **页面名匹配**：必须精确匹配遍历日志中的页面名
4. **同一页面多条规则**：同一页面可配置多条排除规则
5. **动作跳过**：当动作坐标落在排除矩形内时，该动作会被完全跳过
6. **Web 界面限制**：当前 Web 配置界面暂不支持可视化配置 `excluded_touch_areas`，需手动在配置文件中添加
