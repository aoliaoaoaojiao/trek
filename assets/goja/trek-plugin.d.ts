/**
 * Trek Goja 策略插件类型声明
 *
 * 用法：在插件脚本顶部添加
 * /// <reference path="./trek-plugin.d.ts" />
 *
 * 类型声明按功能模块拆分：
 * - types/base.d.ts     基础类型（ActionType, Bounds, LogLevel 等）
 * - types/action.d.ts   动作相关（Action, ActionAPI）
 * - types/page.d.ts     页面相关（Screenshot, PageNode, PageSnapshot, PageAPI）
 * - types/context.d.ts  上下文相关（RuntimeContext, PluginContext, StepResult）
 * - types/plugin.d.ts   插件钩子（Plugin 接口）
 * - types/config.d.ts   配置相关（StaticConfig, UCTBanditConfig）
 * - types/api.d.ts      运行时 API（StoreAPI, LogAPI, HTTPAPI, OCRAPI, LLMAPI, FileAPI）
 */

/// <reference path="types/base.d.ts" />
/// <reference path="types/action.d.ts" />
/// <reference path="types/page.d.ts" />
/// <reference path="types/context.d.ts" />
/// <reference path="types/plugin.d.ts" />
/// <reference path="types/config.d.ts" />
/// <reference path="types/api.d.ts" />

// ── 全局常量 ───────────────────────────────────────────────────

/** Trek 运行时 API：动作构建、页面操作、插件存储、日志 */
declare const trek: Trek.API
/** Trek 静态配置（config.js 中定义） */
declare const config: Trek.StaticConfig
/** Trek 插件钩子（plugin.js 中定义） */
declare const plugin: Trek.Plugin
