package web

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"trek/pkg/driver/android"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common/page/poco"
	"trek/pkg/monkey"
)

//go:embed ui/dist/*
var uiDistFS embed.FS

type webConfigPayload struct {
	PageSource       string `json:"page_source"`
	PageNameStrategy string `json:"page_name_strategy"`
	TouchMode        string `json:"touch_mode"`
	SkipAll          bool   `json:"skip_all_actions_from_model"`
	UIA              struct {
		ServerPort int `json:"server_port"`
	} `json:"uia"`
	Poco struct {
		Engine string `json:"engine"`
		Port   int    `json:"port"`
	} `json:"poco"`
	Log struct {
		FileLevel string `json:"file_level"`
	} `json:"log"`
	EffectiveTouchArea struct {
		Serial      string `json:"serial"`
		PackageName string `json:"package_name"`
		Range       struct {
			Left   float64 `json:"left"`
			Top    float64 `json:"top"`
			Right  float64 `json:"right"`
			Bottom float64 `json:"bottom"`
		} `json:"range"`
	} `json:"effective_touch_area"`
}

type saveRequest struct {
	Config     webConfigPayload `json:"config"`
	OutputPath string           `json:"output_path"`
}

type renderResponse struct {
	JS string `json:"js"`
}

type previewRequest struct {
	Serial string           `json:"serial"`
	Config webConfigPayload `json:"config"`
}

type previewResponse struct {
	UsedSerial       string `json:"used_serial"`
	XML              string `json:"xml"`
	ScreenshotBase64 string `json:"screenshot_base64"`
	PackageName      string `json:"package_name"`
	PageName         string `json:"page_name"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type deviceOption struct {
	Serial string `json:"serial"`
	Label  string `json:"label"`
}

func Serve(addr string) error {
	uiHandler, err := newUIHandler()
	if err != nil {
		return fmt.Errorf("加载前端构建产物失败: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/render", handleRender)
	mux.HandleFunc("/api/save", handleSave)
	mux.HandleFunc("/api/preview", handlePreview)
	mux.HandleFunc("/api/devices", handleDevices)
	mux.Handle("/", uiHandler)

	if strings.TrimSpace(addr) == "" {
		addr = ":17888"
	}
	fmt.Printf("web 配置服务已启动: http://127.0.0.1%s\n", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		return fmt.Errorf("服务启动失败: %w", err)
	}
	return nil
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 GET"})
		return
	}
	adbClient, err := adb.NewClient()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("初始化 adb 客户端失败: %v", err)})
		return
	}

	devices, err := adbClient.DeviceList()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("获取设备列表失败: %v", err)})
		return
	}

	options := make([]deviceOption, 0, len(devices))
	for _, device := range devices {
		serial := strings.TrimSpace(device.Serial())
		if serial == "" {
			continue
		}
		labelParts := make([]string, 0, 3)
		labelParts = append(labelParts, serial)
		if model := strings.TrimSpace(device.Model()); model != "" {
			labelParts = append(labelParts, "model:"+model)
		}
		if product := strings.TrimSpace(device.Product()); product != "" {
			labelParts = append(labelParts, "product:"+product)
		}
		options = append(options, deviceOption{
			Serial: serial,
			Label:  strings.Join(labelParts, " | "),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": options,
	})
}

func newUIHandler() (http.Handler, error) {
	distFS, err := fs.Sub(uiDistFS, "ui/dist")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(distFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		requestPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requestPath == "" || requestPath == "." {
			requestPath = "index.html"
		}

		if _, err := distFS.Open(requestPath); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 POST"})
		return
	}
	cfg, err := decodePayload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	jsText, err := buildConfigJS(cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, renderResponse{JS: jsText})
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 POST"})
		return
	}
	defer r.Body.Close()

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求体不是合法 JSON"})
		return
	}

	path := strings.TrimSpace(req.OutputPath)
	if path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "output_path 不能为空"})
		return
	}
	if strings.ToLower(filepath.Ext(path)) != ".js" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "仅支持保存为 .js 文件"})
		return
	}

	jsText, err := buildConfigJS(req.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "output_path 非法"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("创建目录失败: %v", err)})
		return
	}
	if err := os.WriteFile(absPath, []byte(jsText), 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("保存文件失败: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message":     "保存成功",
		"output_path": absPath,
	})
}

func handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 POST"})
		return
	}
	defer r.Body.Close()

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求体不是合法 JSON"})
		return
	}

	pageSourceType, err := resolvePageSourceType(req.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	driverOptions, err := resolveDriverOptionsForPreview(req.Config, pageSourceType)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	driver, err := android.NewAndroidDriverWith(strings.TrimSpace(req.Serial), driverOptions...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("创建设备驱动失败: %v", err)})
		return
	}
	defer func() { _ = driver.Close() }()

	pageSource := driver.GetPageSource(pageSourceType)
	if pageSource == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("页面源不可用: %s", pageSourceType)})
		return
	}

	xml, err := pageSource.DumpPageSource()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("获取页面 dump 失败: %v", err)})
		return
	}

	screenshot, err := driver.Screenshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("获取截图失败: %v", err)})
		return
	}

	packageName := ""
	if pkg, err := driver.GetCurrentPackage(); err == nil {
		packageName = strings.TrimSpace(pkg)
	}
	activityName := ""
	if previewPageNameStrategyNeedsActivity(req.Config, pageSourceType) {
		if activity, err := driver.GetCurrentActivity(); err == nil {
			activityName = strings.TrimSpace(activity)
		}
	}
	pageName := resolvePreviewPageName(req.Config, pageSourceType, xml, activityName)

	writeJSON(w, http.StatusOK, previewResponse{
		UsedSerial:       strings.TrimSpace(driver.Name()),
		XML:              xml,
		ScreenshotBase64: base64.StdEncoding.EncodeToString(screenshot),
		PackageName:      packageName,
		PageName:         pageName,
	})
}

func resolvePreviewPageName(cfg webConfigPayload, pageSourceType string, xml string, activityName string) string {
	return monkey.ResolvePageNameByStrategy(xml, nil, cfg.PageNameStrategy, pageSourceType, activityName)
}

func previewPageNameStrategyNeedsActivity(cfg webConfigPayload, pageSourceType string) bool {
	strategy := strings.ToLower(strings.TrimSpace(cfg.PageNameStrategy))
	if strategy == "" || strategy == "auto" {
		return strings.EqualFold(strings.TrimSpace(pageSourceType), "uia")
	}
	return strategy == monkey.PageNameStrategyUIAActivityFirst || strategy == monkey.PageNameStrategyActivityOnly
}

func decodePayload(r *http.Request) (webConfigPayload, error) {
	defer r.Body.Close()
	var cfg webConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("请求体不是合法 JSON")
	}
	return cfg, nil
}

func resolvePageSourceType(cfg webConfigPayload) (string, error) {
	pageSource := strings.ToLower(strings.TrimSpace(cfg.PageSource))
	if pageSource == "" {
		pageSource = "uia"
	}
	switch pageSource {
	case "uia", "poco":
		return pageSource, nil
	default:
		return "", fmt.Errorf("不支持的页面源: %s（可选: uia, poco）", pageSource)
	}
}

func resolveTouchType(cfg webConfigPayload) (android.TouchType, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.TouchMode))
	if mode == "" {
		mode = "motion"
	}
	switch mode {
	case "motion":
		return android.TouchTypeMotion, nil
	case "uia":
		return android.TouchTypeUIA, nil
	case "adb":
		return android.TouchTypeADB, nil
	default:
		return "", fmt.Errorf("不支持的触控模式: %s（可选: motion, uia, adb）", mode)
	}
}

func resolveDriverOptionsForPreview(cfg webConfigPayload, pageSourceType string) ([]android.AndroidDriverOption, error) {
	touchType, err := resolveTouchType(cfg)
	if err != nil {
		return nil, err
	}

	options := []android.AndroidDriverOption{
		android.WithTouch(touchType),
	}

	if cfg.UIA.ServerPort > 0 {
		options = append(options, android.WithUIAServerPort(cfg.UIA.ServerPort))
	}

	if pageSourceType == "poco" {
		engineText := strings.TrimSpace(cfg.Poco.Engine)
		if engineText == "" {
			engineText = "UNITY_3D"
		}
		engine, err := parsePocoEngine(engineText)
		if err != nil {
			return nil, err
		}
		pocoPort := cfg.Poco.Port
		if pocoPort <= 0 {
			pocoPort = engine.GetDefaultPort()
		}
		if pocoPort <= 0 {
			return nil, fmt.Errorf("Poco 端口无效，请配置 poco.port")
		}
		options = append(options, android.WithPoco(engine, pocoPort))
	}

	return options, nil
}

func parsePocoEngine(text string) (poco.Engine, error) {
	raw := strings.TrimSpace(text)
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(raw, "-", "_"), " ", "_"))
	switch normalized {
	case string(poco.Unity3d), "UNITY", "UNITY3D":
		return poco.Unity3d, nil
	case string(poco.UE4):
		return poco.UE4, nil
	case string(poco.Cocos2dxJs), "COCOS2DX_JS", "COCOS_JS":
		return poco.Cocos2dxJs, nil
	case string(poco.CocosCreator), "COCOS_CREATOR3D":
		return poco.CocosCreator, nil
	case string(poco.Egret):
		return poco.Egret, nil
	case string(poco.Cocos2dxLua), "COCOS2DX_LUA":
		return poco.Cocos2dxLua, nil
	case string(poco.Cocos2dxCPlus), "COCOS2DX_C++", "COCOS2DX_CPLUS", "COCOS2DX_CPP":
		return poco.Cocos2dxCPlus, nil
	default:
		return "", fmt.Errorf("不支持的 Poco 引擎: %s", raw)
	}
}

func buildConfigJS(cfg webConfigPayload) (string, error) {
	pageSource := strings.ToLower(strings.TrimSpace(cfg.PageSource))
	if pageSource == "" {
		pageSource = "uia"
	}
	if pageSource != "uia" && pageSource != "poco" {
		return "", fmt.Errorf("page_source 仅支持 uia 或 poco")
	}

	touchMode := strings.ToLower(strings.TrimSpace(cfg.TouchMode))
	if touchMode == "" {
		touchMode = "motion"
	}
	if touchMode != "motion" && touchMode != "uia" && touchMode != "adb" {
		return "", fmt.Errorf("touch_mode 仅支持 motion/uia/adb")
	}

	pageNameStrategy := strings.ToLower(strings.TrimSpace(cfg.PageNameStrategy))
	if pageNameStrategy != "" {
		switch pageNameStrategy {
		case "uia_activity_first", "xml_only", "xml_fingerprint", "structure_fingerprint", "activity_only":
		default:
			return "", fmt.Errorf("page_name_strategy 不合法: %s", pageNameStrategy)
		}
	}

	if cfg.UIA.ServerPort < 0 || cfg.Poco.Port < 0 {
		return "", fmt.Errorf("端口不能为负数")
	}
	if cfg.EffectiveTouchArea.Range.Right < cfg.EffectiveTouchArea.Range.Left ||
		cfg.EffectiveTouchArea.Range.Bottom < cfg.EffectiveTouchArea.Range.Top {
		return "", fmt.Errorf("effective_touch_area.range 要求 right>=left 且 bottom>=top")
	}

	pocoEngine := strings.TrimSpace(strings.ToUpper(cfg.Poco.Engine))
	if pageSource == "poco" && pocoEngine == "" {
		pocoEngine = "UNITY_3D"
	}
	if pocoEngine != "" {
		validEngines := map[string]bool{
			"UNITY_3D":       true,
			"UE4":            true,
			"COCOS2DX_JS":    true,
			"COCOS_CREATOR":  true,
			"EGRET":          true,
			"COCOS2DX_LUA":   true,
			"COCOS2DX_CPLUS": true,
		}
		if !validEngines[pocoEngine] {
			return "", fmt.Errorf("poco.engine 不合法: %s", pocoEngine)
		}
	}

	var b strings.Builder
	b.WriteString("const config = {\n")
	b.WriteString(fmt.Sprintf("  page_source: %q,\n", pageSource))
	if pageNameStrategy != "" {
		b.WriteString(fmt.Sprintf("  page_name_strategy: %q,\n", pageNameStrategy))
	}
	b.WriteString(fmt.Sprintf("  touch_mode: %q,\n", touchMode))
	if cfg.SkipAll {
		b.WriteString("  skip_all_actions_from_model: true,\n")
	}

	if cfg.UIA.ServerPort > 0 {
		b.WriteString("  uia: {\n")
		b.WriteString("    server_port: " + strconv.Itoa(cfg.UIA.ServerPort) + ",\n")
		b.WriteString("  },\n")
	}

	if pageSource == "poco" || pocoEngine != "" || cfg.Poco.Port > 0 {
		b.WriteString("  poco: {\n")
		if pocoEngine != "" {
			b.WriteString(fmt.Sprintf("    engine: %q,\n", pocoEngine))
		}
		if cfg.Poco.Port > 0 {
			b.WriteString("    port: " + strconv.Itoa(cfg.Poco.Port) + ",\n")
		}
		b.WriteString("  },\n")
	}

	fileLevel := strings.ToLower(strings.TrimSpace(cfg.Log.FileLevel))
	if fileLevel != "" {
		switch fileLevel {
		case "debug", "info", "warn", "error":
			b.WriteString("  log: {\n")
			b.WriteString(fmt.Sprintf("    file_level: %q,\n", fileLevel))
			b.WriteString("  },\n")
		default:
			return "", fmt.Errorf("log.file_level 仅支持 debug/info/warn/error")
		}
	}

	areaSerial := strings.TrimSpace(cfg.EffectiveTouchArea.Serial)
	areaPackage := strings.TrimSpace(cfg.EffectiveTouchArea.PackageName)
	areaRange := cfg.EffectiveTouchArea.Range
	hasAreaRange := areaRange.Left != 0 || areaRange.Top != 0 || areaRange.Right != 0 || areaRange.Bottom != 0
	if hasAreaRange || areaSerial != "" || areaPackage != "" {
		if !hasAreaRange {
			areaRange.Left = 0
			areaRange.Top = 0
			areaRange.Right = 1
			areaRange.Bottom = 1
		}
		b.WriteString("  effective_touch_area: {\n")
		if areaSerial != "" {
			b.WriteString(fmt.Sprintf("    serial: %q,\n", areaSerial))
		}
		if areaPackage != "" {
			b.WriteString(fmt.Sprintf("    package_name: %q,\n", areaPackage))
		}
		b.WriteString("    range: {\n")
		b.WriteString(fmt.Sprintf("      left: %s,\n", strconv.FormatFloat(areaRange.Left, 'f', -1, 64)))
		b.WriteString(fmt.Sprintf("      top: %s,\n", strconv.FormatFloat(areaRange.Top, 'f', -1, 64)))
		b.WriteString(fmt.Sprintf("      right: %s,\n", strconv.FormatFloat(areaRange.Right, 'f', -1, 64)))
		b.WriteString(fmt.Sprintf("      bottom: %s,\n", strconv.FormatFloat(areaRange.Bottom, 'f', -1, 64)))
		b.WriteString("    },\n")
		b.WriteString("  },\n")
	}

	b.WriteString("}\n")
	return b.String(), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
