/**
 * Trek 运行时 API 类型定义
 */

declare namespace Trek {
  /**
   * 跨步骤持久键值存储（插件私有，不参与引擎内部决策）。
   * key 为插件脚本中自定义的任意字符串，无预定义 schema，按需命名即可。
   * 同一插件生命周期内（onInit → onDestroy），所有钩子共享此状态；
   * 适合记录访问计数、决策历史、页面标记等策略数据。
   * 多插件时各插件实例独立，互不影响。
   *
   * 注意：引擎决策层使用的页面访问计数、动作计数等数据在 ctx.runtime 中提供，
   * 与 trek.store 互不干扰。
   */
  interface StoreAPI {
    /**
     * 读取持久状态值，key 不存在返回 undefined。
     * @example
     * const count = trek.store.get('click_count') || 0;
     */
    get<T = unknown>(key: string): T | undefined
    /**
     * 写入持久状态值，跨步骤可用。
     * @example
     * trek.store.set('last_page', ctx.page.page_name);
     */
    set<T = unknown>(key: string, value: T): void
    /**
     * 对指定 key 做整数自增（默认 +1），返回自增后的值。
     * @example
     * const n = trek.store.increment('visit_count');
     * trek.store.increment('score', 10);
     */
    increment(key: string, delta?: number): number
    /**
     * 删除指定 key。
     * @example
     * trek.store.delete('temp_data');
     */
    delete(key: string): void
    /**
     * 清空所有持久状态。
     * @example
     * trek.store.clear();
     */
    clear(): void
  }

  interface LogAPI {
    debug(message: string): void
    info(message: string): void
    warn(message: string): void
    error(message: string): void
  }

  interface HTTPRequestOptions {
    method?: string
    url: string
    headers?: Record<string, string>
    body?: string | Uint8Array | number[]
    /** 请求超时（毫秒）。默认 10000，最大 30000 */
    timeout_ms?: number
  }

  interface HTTPResponse {
    status: number
    status_text: string
    ok: boolean
    headers: Record<string, string>
    body: string
    bytes: Uint8Array
  }

  interface HTTPAPI {
    /** 发起同步 HTTP 请求，仅支持 http / https。响应体最大 2MB。 */
    request(options: HTTPRequestOptions): HTTPResponse
    /** 发起 GET 请求。 */
    get(url: string, options?: Omit<HTTPRequestOptions, "method" | "url" | "body">): HTTPResponse
    /** 发起 POST 请求。 */
    post(url: string, body?: string | Uint8Array | number[], options?: Omit<HTTPRequestOptions, "method" | "url" | "body">): HTTPResponse
  }

  // ── OCR API ──────────────────────────────────────────────────

  /** OCR 识别出的文本区域。 */
  interface OCRRegion {
    /** 识别出的文本内容（格式为 intent 字符串，如 "ocr_click:确定"）。 */
    text: string
    /** 置信度 [0, 1]。 */
    confidence: number
    /** 归一化边界 [left, top, right, bottom]，范围 [0, 1]。 */
    bounds: [number, number, number, number]
  }

  interface OCRRecognizeOptions {
    /** 截图字节（来自 trek.page.screenshotBytes）。 */
    screenshot: Uint8Array | number[]
    /** OCR 服务端点 URL。缺省读 PADDLEOCR_API_URL 环境变量。 */
    endpoint?: string
    /** 认证密钥。缺省读 PADDLEOCR_API_KEY 环境变量。 */
    api_key?: string
    /** 请求超时毫秒。默认 10000。 */
    timeout_ms?: number
    /** 额外请求头。 */
    headers?: Record<string, string>
  }

  interface OCRAPI {
    /**
     * 调用 OCR 服务识别截图中的文本区域，返回归一化坐标的区域列表。
     * @example
     * const regions = trek.ocr.recognize({
     *   screenshot: trek.page.screenshotBytes(ctx.page),
     *   endpoint: 'http://ocr-server:8080/ocr',
     * });
     * for (const r of regions) {
     *   trek.log.info(`text=${r.text} bounds=${r.bounds}`);
     * }
     */
    recognize(options: OCRRecognizeOptions): OCRRegion[]
  }

  // ── LLM API ──────────────────────────────────────────────────

  interface LLMChatOptions {
    /** 用户提示词。 */
    prompt: string
    /** 可选截图（多模态输入）。 */
    screenshot?: Uint8Array | number[]
    /** LLM 端点 URL（OpenAI Chat Completions 格式）。缺省读 LLM_API_URL / OPENAI_API_URL。 */
    endpoint?: string
    /** 认证密钥。缺省读 LLM_API_KEY / OPENAI_API_KEY。 */
    api_key?: string
    /** 模型名称。缺省读 LLM_MODEL / OPENAI_MODEL。 */
    model?: string
    /** 请求超时毫秒。默认 30000。 */
    timeout_ms?: number
    /** 额外请求头。 */
    headers?: Record<string, string>
    /** 最大输出 token 数。默认 4096。 */
    max_tokens?: number
  }

  interface LLMAPI {
    /**
     * 调用 LLM 多模态对话，返回文本响应。
     * 支持所有 OpenAI Chat Completions 格式的端点（GPT-4o、Gemini、Qwen-VL 等）。
     * @example
     * const text = trek.llm.chat({
     *   prompt: '根据截图描述当前页面内容',
     *   screenshot: trek.page.screenshotBytes(ctx.page),
     * });
     */
    chat(options: LLMChatOptions): string
  }

  // ── File API ─────────────────────────────────────────────────

  /** trek.file.open() 返回的文件句柄。 */
  interface FileHandle {
    /** 读取全部内容为字符串。 */
    readString(): string
    /** 读取全部内容为字节数组。 */
    readBytes(): Uint8Array
    /** 读取 n 字节；n<=0 或省略则读取全部。 */
    read(n?: number): Uint8Array
    /** 读取一行（不含换行符）。 */
    readLine(): string
    /** 读取所有行，返回字符串数组。 */
    readLines(): string[]
    /** 写入字符串，返回写入字节数。 */
    writeString(data: string): number
    /** 写入字节数组，返回写入字节数。 */
    writeBytes(data: Uint8Array | number[]): number
    /** 写入字符串或字节，返回写入字节数。 */
    write(data: string | Uint8Array | number[]): number
    /** 移动文件指针。whence: "start"（默认）/ "current" / "end"。 */
    seek(offset: number, whence?: string): number
    /** 返回当前文件指针位置。 */
    tell(): number
    /** 返回文件大小（字节）。 */
    size(): number
    /** 关闭文件句柄。 */
    close(): void
    /** 返回文件路径。 */
    path(): string
  }

  interface FileAPI {
    /**
     * 打开文件，返回文件句柄。
     * @param path 文件路径
     * @param mode 打开模式："r" 只读（默认）, "w" 写入（清空）, "a" 追加, "r+" 读写
     * @example
     * const f = trek.file.open('/sdcard/config.json');
     * const text = f.readString();
     * f.close();
     *
     * const w = trek.file.open('/sdcard/output.txt', 'w');
     * w.writeString('hello\n');
     * w.close();
     */
    open(path: string, mode?: string): FileHandle
    /**
     * 检查文件是否存在。
     * @example
     * if (trek.file.exists('/sdcard/config.json')) { ... }
     */
    exists(path: string): boolean
  }

  export interface API {
    action: ActionAPI
    page: PageAPI
    store: StoreAPI
    log: LogAPI
    http: HTTPAPI
    ocr: OCRAPI
    llm: LLMAPI
    file: FileAPI
    /** 同步暂停指定毫秒数，最大 30000。 */
    sleep(milliseconds: number): void
  }
}
