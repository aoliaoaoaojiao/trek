/**
 * Trek 页面相关类型定义
 */

declare namespace Trek {
  interface Screenshot {
    /** PNG/JPEG 原始字节 */
    bytes: Uint8Array
    /** MIME 类型 */
    mime: "image/png" | "image/jpeg"
    /** 字节数 */
    size: number
    /** 图片宽度 */
    width?: number
    /** 图片高度 */
    height?: number
  }

  interface PageNode {
    /** 节点文本（text） */
    text: string
    /** 资源 ID（resource-id） */
    resource_id: string
    /** 无障碍描述（content-desc） */
    content_desc: string
    /** 节点类名（class） */
    class_name: string
    /** 节点边界 [left, top, right, bottom] */
    bounds: Bounds
    /** 是否可点击 */
    clickable: boolean
    /** 是否可用 */
    enabled: boolean
    /** 是否可编辑输入 */
    editable: boolean
    /** 节点的标准 XPath（用于跨模块定位和调试） */
    xpath?: string
  }

  /** 页面快照：由引擎采集并传给插件。 */
  interface PageSnapshot {
    /**
     * 当前页面名。
     * 当 page_source_type="uia" 时，默认优先使用 `dumpsys activity top` 解析出的 Activity 名。
     * 插件可在 `transformPage` 中返回 `page_name` 进行覆盖。
     */
    page_name: string
    /** 当前页面 XML（插件可在 transformPage 中返回 xml 覆盖） */
    xml: string
    /** 截图（需运行时开启截图采集） */
    screenshot?: Screenshot
    /** 从 XML 提取的节点列表，便于脚本筛选控件 */
    nodes: PageNode[]
  }

  interface PageInfo {
    /** 覆盖页面名；不返回则沿用原值 */
    page_name?: string
    /** 覆盖页面 XML；不返回则沿用原值 */
    xml?: string
  }

  interface PageAPI {
    /**
     * 通过标准 XPath 从页面 XML 中查找节点，返回 PageNode 或 null。
     * @example
     * const node = trek.page.findByXpath(ctx.page, '//node[@resource-id="com.example:id/btn"]');
     * if (node) trek.log.info(`found: ${node.text}`);
     */
    findByXpath(page: PageSnapshot, xpath: string): PageNode | null
    /**
     * 从 XML 中移除所有包含指定文本的节点（字符串精确匹配）。
     * @example
     * ctx.page.xml = trek.page.excludeByText(ctx.page.xml, '广告');
     */
    excludeByText(xml: string, text: string): string
    /**
     * 从 XML 中移除所有包含指定 resource-id 的节点（字符串精确匹配）。
     * @example
     * ctx.page.xml = trek.page.excludeByResourceId(ctx.page.xml, 'com.example:id/ad_container');
     */
    excludeByResourceId(xml: string, id: string): string
    /**
     * 替换 XML 中的文本；from 支持字符串或正则（如 /pattern/flags）。
     * @example
     * ctx.page.xml = trek.page.replaceText(ctx.page.xml, '登录', 'Sign In');
     * ctx.page.xml = trek.page.replaceText(ctx.page.xml, /用户\d+/g, '用户***');
     */
    replaceText(xml: string, from: string | RegExp, to: string): string
    /**
     * 替换 XML 中的 resource-id；from 支持字符串或正则。
     * @example
     * ctx.page.xml = trek.page.replaceResourceId(ctx.page.xml, 'com.example.v2', 'com.example');
     */
    replaceResourceId(xml: string, from: string | RegExp, to: string): string
    /**
     * 判断页面快照是否包含截图。
     * @example
     * if (trek.page.hasScreenshot(ctx.page)) { ... }
     */
    hasScreenshot(page: PageSnapshot): boolean
    /**
     * 获取截图原始字节，无截图时返回 null。
     * @example
     * const bytes = trek.page.screenshotBytes(ctx.page);
     * if (bytes) trek.log.info(`size: ${bytes.length}`);
     */
    screenshotBytes(page: PageSnapshot): Uint8Array | null
    /**
     * 获取截图字节数，无截图时返回 0。
     * @example
     * trek.log.info(`screenshot: ${trek.page.screenshotSize(ctx.page)} bytes`);
     */
    screenshotSize(page: PageSnapshot): number
  }
}
