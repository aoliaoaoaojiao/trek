/*
Copyright © 2026 Trek

Package webserver 提供 Trek Web 配置界面的 HTTP 服务端逻辑。
*/
package webserver

import (
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

	"trek/internal/config"
	"trek/pkg/driver/android"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common/page/poco"
	"trek/pkg/monkey"
)

// ConfigPayload 是 web 配置界面的 JSON 请求体结构。
type ConfigPayload struct {
	PageSource                        string   `json:"page_source"`
	PageNameStrategy                  string   `json:"page_name_strategy"`
	TouchMode                         string   `json:"touch_mode"`
	SkipAll                           bool     `json:"skip_all_actions_from_model"`
	PageControl                       string   `json:"page_control_strategy"`
	Algorithm                         string   `json:"algorithm"`
	CaptureScreenshot                 *bool    `json:"capture_screenshot"`
	KeepStepRecords                   *bool    `json:"keep_step_records"`
	ScrollInferThreshold              *int     `json:"scroll_infer_threshold"`
	ImageSimilaritySSIMThreshold      *float64 `json:"image_similarity_ssim_threshold"`
	ExploreOCRTimeoutMs               *int     `json:"explore_ocr_timeout_ms"`
	LLMTimeoutMs                      *int     `json:"llm_timeout_ms"`
	RecoveryCooldownSteps             *int     `json:"recovery_cooldown_steps"`
	RecoveryTwoStateLoopThreshold     *int     `json:"recovery_two_state_loop_threshold"`
	RecoveryHighVisitThreshold        *int     `json:"recovery_high_visit_threshold"`
	RecoveryLowRewardWindow           *int     `json:"recovery_low_reward_window"`
	CandidateAmbiguityTopGapThreshold *float64 `json:"candidate_ambiguity_top_gap_threshold"`
	HighValuePageVisitLimit           *int     `json:"high_value_page_visit_limit"`
	CandidateRiskDropThreshold        *float64 `json:"candidate_risk_drop_threshold"`
	CandidateMinFusionScore           *float64 `json:"candidate_min_fusion_score"`
	UIA                               struct {
		ServerPort int `json:"server_port"`
	} `json:"uia"`
	Poco struct {
		Engine string `json:"engine"`
		Port   int    `json:"port"`
	} `json:"poco"`
	Log struct {
		FileLevel string `json:"file_level"`
	} `json:"log"`
	UCTBandit struct {
		TwoStateLoopPenalty    *float64 `json:"two_state_loop_penalty"`
		EdgeRepeatPenalty      *float64 `json:"edge_repeat_penalty"`
		EdgeRepeatThreshold    *int     `json:"edge_repeat_threshold"`
		ActionCooldownPenalty  *float64 `json:"action_cooldown_penalty"`
		RecentActionWindow     *int     `json:"recent_action_window"`
		LoopEscapeExploreBoost *float64 `json:"loop_escape_explore_boost"`
	} `json:"uct_bandit"`
	Reuse struct {
		Epsilon                *float64 `json:"epsilon"`
		Gamma                  *float64 `json:"gamma"`
		NStep                  *int     `json:"n_step"`
		ModelSavePath          string   `json:"model_save_path"`
		EnableModelPersistence *bool    `json:"enable_model_persistence"`
		ResetModelOnStart      *bool    `json:"reset_model_on_start"`
	} `json:"reuse"`
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

// SaveRequest 是保存配置的请求体。
type SaveRequest struct {
	Config     ConfigPayload `json:"config"`
	OutputPath string        `json:"output_path"`
}

// RenderResponse 是渲染配置的响应体。
type RenderResponse struct {
	JS string `json:"js"`
}

// PreviewRequest 是预览页面的请求体。
type PreviewRequest struct {
	Serial string        `json:"serial"`
	Config ConfigPayload `json:"config"`
}

// PreviewResponse 是预览页面的响应体。
type PreviewResponse struct {
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

// Serve 启动 Web 配置服务。
// uiFS 是通过 go:embed 嵌入的前端构建产物文件系统。
func Serve(addr string, uiFS fs.FS) error {
	uiHandler, err := newUIHandler(uiFS)
	if err != nil {
		return fmt.Errorf("加载前端构建产物失败: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/render", handleRender)
	mux.HandleFunc("/api/save", handleSave)
	mux.HandleFunc("/api/preview", handlePreview)
	mux.HandleFunc("/api/devices", handleDevices)
	mux.HandleFunc("/api/defaults", handleDefaults)
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

// DefaultsResponse 返回后端默认配置值，供前端动态显示。
type DefaultsResponse struct {
	ScrollInferThreshold             int     `json:"scroll_infer_threshold"`
	ImageSimilaritySSIMThreshold     float64 `json:"image_similarity_ssim_threshold"`
	ImageFingerprintHammingThreshold int     `json:"image_fingerprint_hamming_threshold"`
	PageControlCacheTTLSeconds       int     `json:"page_control_cache_ttl_seconds"`
	ExploreOCRTimeoutMs              int     `json:"explore_ocr_timeout_ms"`
	LLMTimeoutMs                     int     `json:"llm_timeout_ms"`
	TwoStateLoopPenalty              float64 `json:"two_state_loop_penalty"`
	EdgeRepeatPenalty                float64 `json:"edge_repeat_penalty"`
	EdgeRepeatThreshold              int     `json:"edge_repeat_threshold"`
	ActionCooldownPenalty            float64 `json:"action_cooldown_penalty"`
	RecentActionWindow               int     `json:"recent_action_window"`
	LoopEscapeExploreBoost           float64 `json:"loop_escape_explore_boost"`
	BackPenalty                      float64 `json:"back_penalty"`
	ShortLoopPenalty                 float64 `json:"short_loop_penalty"`
	ShortLoopWindow                  int     `json:"short_loop_window"`
	NewStateReward                   float64 `json:"new_state_reward"`
}

func handleDefaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 GET"})
		return
	}

	writeJSON(w, http.StatusOK, DefaultsResponse{
		ScrollInferThreshold:             config.DefaultScrollInferThreshold,
		ImageSimilaritySSIMThreshold:     config.DefaultImageSimilaritySSIMThreshold,
		ImageFingerprintHammingThreshold: config.DefaultImageFingerprintHammingThreshold,
		PageControlCacheTTLSeconds:       config.DefaultPageControlCacheTTLSeconds,
		ExploreOCRTimeoutMs:              config.DefaultExploreOCRTimeoutMs,
		LLMTimeoutMs:                     config.DefaultLLMTimeoutMs,
		TwoStateLoopPenalty:              config.DefaultTwoStateLoopPenalty,
		EdgeRepeatPenalty:                config.DefaultEdgeRepeatPenalty,
		EdgeRepeatThreshold:              config.DefaultEdgeRepeatThreshold,
		ActionCooldownPenalty:            config.DefaultActionCooldownPenalty,
		RecentActionWindow:               config.DefaultRecentActionWindow,
		LoopEscapeExploreBoost:           config.DefaultLoopEscapeExploreBoost,
		BackPenalty:                      config.DefaultBackPenalty,
		ShortLoopPenalty:                 config.DefaultShortLoopPenalty,
		ShortLoopWindow:                  config.DefaultShortLoopWindow,
		NewStateReward:                   config.DefaultNewStateReward,
	})
}

func newUIHandler(uiFS fs.FS) (http.Handler, error) {
	fileServer := http.FileServer(http.FS(uiFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		requestPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requestPath == "" || requestPath == "." {
			requestPath = "index.html"
		}

		if _, err := uiFS.Open(requestPath); err != nil {
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
	jsText, err := BuildConfigJS(cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, RenderResponse{JS: jsText})
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "仅支持 POST"})
		return
	}
	defer r.Body.Close()

	var req SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求体不是合法 JSON"})
		return
	}

	p := strings.TrimSpace(req.OutputPath)
	if p == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "output_path 不能为空"})
		return
	}
	if strings.ToLower(filepath.Ext(p)) != ".js" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "仅支持保存为 .js 文件"})
		return
	}

	jsText, err := BuildConfigJS(req.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	absPath, err := filepath.Abs(p)
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

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请求体不是合法 JSON"})
		return
	}

	pageSourceType, err := android.ResolvePageSourceType(req.Config.PageSource)
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

	xml := ""
	if pageSourceType != "screenshot" {
		pageSource := driver.GetPageSource(pageSourceType)
		if pageSource == nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("页面源不可用: %s", pageSourceType)})
			return
		}

		xml, err = pageSource.DumpPageSource()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("获取页面 dump 失败: %v", err)})
			return
		}
	}

	screenshot, err := driver.Screenshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: fmt.Sprintf("获取截图失败: %v", err)})
		return
	}

	packageName := ""
	if pkg, err := driver.GetCurrentPackage(r.Context()); err == nil {
		packageName = strings.TrimSpace(pkg)
	}
	activityName := ""
	if previewPageNameStrategyNeedsActivity(req.Config, pageSourceType) {
		if activity, err := driver.GetCurrentActivity(r.Context()); err == nil {
			activityName = strings.TrimSpace(activity)
		}
	}
	pageName := resolvePreviewPageName(req.Config, pageSourceType, xml, screenshot, activityName)

	writeJSON(w, http.StatusOK, PreviewResponse{
		UsedSerial:       strings.TrimSpace(driver.Name()),
		XML:              xml,
		ScreenshotBase64: base64.StdEncoding.EncodeToString(screenshot),
		PackageName:      packageName,
		PageName:         pageName,
	})
}

// --- 辅助函数 ---

func resolvePreviewPageName(cfg ConfigPayload, pageSourceType string, xml string, screenshot []byte, activityName string) string {
	if strings.EqualFold(strings.TrimSpace(cfg.PageNameStrategy), monkey.PageNameStrategyImageFingerprint) {
		return monkey.ResolveImageFingerprintPageName(screenshot, nil)
	}
	return monkey.ResolvePageNameByStrategy(xml, nil, cfg.PageNameStrategy, pageSourceType, activityName)
}

func previewPageNameStrategyNeedsActivity(cfg ConfigPayload, pageSourceType string) bool {
	strategy := strings.ToLower(strings.TrimSpace(cfg.PageNameStrategy))
	if strategy == "" || strategy == "auto" {
		return false
	}
	return strategy == monkey.PageNameStrategyActivityOnly
}

func decodePayload(r *http.Request) (ConfigPayload, error) {
	defer r.Body.Close()
	var cfg ConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("请求体不是合法 JSON")
	}
	return cfg, nil
}

func resolveDriverOptionsForPreview(cfg ConfigPayload, pageSourceType string) ([]android.AndroidDriverOption, error) {
	_, touchType, err := android.ResolveTouchMode(cfg.TouchMode)
	if err != nil {
		return nil, err
	}

	engineText := strings.TrimSpace(cfg.Poco.Engine)
	if pageSourceType == "poco" && engineText == "" {
		engineText = "UNITY_3D"
	}
	return android.BuildDriverOptions(android.DriverBootstrapConfig{
		PageSource:    cfg.PageSource,
		TouchMode:     cfg.TouchMode,
		UIAServerPort: cfg.UIA.ServerPort,
		PocoEngine:    engineText,
		PocoPort:      cfg.Poco.Port,
	}, pageSourceType, touchType)
}

// ParsePocoEngine 将字符串解析为 Poco 引擎类型（导出供 cmd 包使用）。
func ParsePocoEngine(text string) (poco.Engine, error) {
	return android.ParsePocoEngine(text)
}

// BuildConfigJS 从配置载荷生成 JS 配置脚本（导出供 cmd 包和测试使用）。
func BuildConfigJS(cfg ConfigPayload) (string, error) {
	pageSource := strings.ToLower(strings.TrimSpace(cfg.PageSource))
	if pageSource == "" {
		pageSource = "uia"
	}
	if pageSource != "uia" && pageSource != "poco" && pageSource != "screenshot" {
		return "", fmt.Errorf("page_source 仅支持 uia / poco / screenshot")
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
		case "structure_fingerprint", "activity_only", "image_fingerprint":
		default:
			return "", fmt.Errorf("page_name_strategy 不合法: %s", pageNameStrategy)
		}
	}
	pageControlStrategy := strings.ToLower(strings.TrimSpace(cfg.PageControl))
	if pageControlStrategy != "" {
		switch pageControlStrategy {
		case "raw", "ocr", "llm":
		default:
			return "", fmt.Errorf("page_control_strategy 不合法: %s", pageControlStrategy)
		}
	}
	if pageSource == "screenshot" && (pageControlStrategy == "" || pageControlStrategy == "raw") {
		pageControlStrategy = "ocr"
	}
	algorithm := strings.ToLower(strings.TrimSpace(cfg.Algorithm))
	if algorithm != "" {
		switch algorithm {
		case "reuse", "uctbandit", "random":
		default:
			return "", fmt.Errorf("algorithm 不合法: %s", algorithm)
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
	if pageControlStrategy != "" {
		b.WriteString(fmt.Sprintf("  page_control_strategy: %q,\n", pageControlStrategy))
	}
	if algorithm != "" {
		b.WriteString(fmt.Sprintf("  algorithm: %q,\n", algorithm))
	}
	if cfg.SkipAll {
		b.WriteString("  skip_all_actions_from_model: true,\n")
	}
	if pageSource == "screenshot" {
		b.WriteString("  capture_screenshot: true,\n")
	} else if cfg.CaptureScreenshot != nil {
		b.WriteString(fmt.Sprintf("  capture_screenshot: %t,\n", *cfg.CaptureScreenshot))
	}
	if cfg.KeepStepRecords != nil {
		b.WriteString(fmt.Sprintf("  keep_step_records: %t,\n", *cfg.KeepStepRecords))
	}
	if cfg.ScrollInferThreshold != nil {
		b.WriteString(fmt.Sprintf("  scroll_infer_threshold: %d,\n", *cfg.ScrollInferThreshold))
	}
	if cfg.ImageSimilaritySSIMThreshold != nil {
		b.WriteString(fmt.Sprintf("  image_similarity_ssim_threshold: %s,\n", strconv.FormatFloat(*cfg.ImageSimilaritySSIMThreshold, 'f', -1, 64)))
	}
	if cfg.ExploreOCRTimeoutMs != nil {
		b.WriteString(fmt.Sprintf("  explore_ocr_timeout_ms: %d,\n", *cfg.ExploreOCRTimeoutMs))
	}
	if cfg.LLMTimeoutMs != nil {
		b.WriteString(fmt.Sprintf("  llm_timeout_ms: %d,\n", *cfg.LLMTimeoutMs))
	}
	if cfg.RecoveryCooldownSteps != nil {
		b.WriteString(fmt.Sprintf("  recovery_cooldown_steps: %d,\n", *cfg.RecoveryCooldownSteps))
	}
	if cfg.RecoveryTwoStateLoopThreshold != nil {
		b.WriteString(fmt.Sprintf("  recovery_two_state_loop_threshold: %d,\n", *cfg.RecoveryTwoStateLoopThreshold))
	}
	if cfg.RecoveryHighVisitThreshold != nil {
		b.WriteString(fmt.Sprintf("  recovery_high_visit_threshold: %d,\n", *cfg.RecoveryHighVisitThreshold))
	}
	if cfg.RecoveryLowRewardWindow != nil {
		b.WriteString(fmt.Sprintf("  recovery_low_reward_window: %d,\n", *cfg.RecoveryLowRewardWindow))
	}
	if cfg.CandidateAmbiguityTopGapThreshold != nil {
		b.WriteString(fmt.Sprintf("  candidate_ambiguity_top_gap_threshold: %s,\n", strconv.FormatFloat(*cfg.CandidateAmbiguityTopGapThreshold, 'f', -1, 64)))
	}
	if cfg.HighValuePageVisitLimit != nil {
		b.WriteString(fmt.Sprintf("  high_value_page_visit_limit: %d,\n", *cfg.HighValuePageVisitLimit))
	}
	if cfg.CandidateRiskDropThreshold != nil {
		b.WriteString(fmt.Sprintf("  candidate_risk_drop_threshold: %s,\n", strconv.FormatFloat(*cfg.CandidateRiskDropThreshold, 'f', -1, 64)))
	}
	if cfg.CandidateMinFusionScore != nil {
		b.WriteString(fmt.Sprintf("  candidate_min_fusion_score: %s,\n", strconv.FormatFloat(*cfg.CandidateMinFusionScore, 'f', -1, 64)))
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

	if cfg.UCTBandit.TwoStateLoopPenalty != nil ||
		cfg.UCTBandit.EdgeRepeatPenalty != nil ||
		cfg.UCTBandit.EdgeRepeatThreshold != nil ||
		cfg.UCTBandit.ActionCooldownPenalty != nil ||
		cfg.UCTBandit.RecentActionWindow != nil ||
		cfg.UCTBandit.LoopEscapeExploreBoost != nil {
		b.WriteString("  uct_bandit: {\n")
		if cfg.UCTBandit.TwoStateLoopPenalty != nil {
			b.WriteString(fmt.Sprintf("    two_state_loop_penalty: %s,\n", strconv.FormatFloat(*cfg.UCTBandit.TwoStateLoopPenalty, 'f', -1, 64)))
		}
		if cfg.UCTBandit.EdgeRepeatPenalty != nil {
			b.WriteString(fmt.Sprintf("    edge_repeat_penalty: %s,\n", strconv.FormatFloat(*cfg.UCTBandit.EdgeRepeatPenalty, 'f', -1, 64)))
		}
		if cfg.UCTBandit.EdgeRepeatThreshold != nil {
			b.WriteString(fmt.Sprintf("    edge_repeat_threshold: %d,\n", *cfg.UCTBandit.EdgeRepeatThreshold))
		}
		if cfg.UCTBandit.ActionCooldownPenalty != nil {
			b.WriteString(fmt.Sprintf("    action_cooldown_penalty: %s,\n", strconv.FormatFloat(*cfg.UCTBandit.ActionCooldownPenalty, 'f', -1, 64)))
		}
		if cfg.UCTBandit.RecentActionWindow != nil {
			b.WriteString(fmt.Sprintf("    recent_action_window: %d,\n", *cfg.UCTBandit.RecentActionWindow))
		}
		if cfg.UCTBandit.LoopEscapeExploreBoost != nil {
			b.WriteString(fmt.Sprintf("    loop_escape_explore_boost: %s,\n", strconv.FormatFloat(*cfg.UCTBandit.LoopEscapeExploreBoost, 'f', -1, 64)))
		}
		b.WriteString("  },\n")
	}

	if cfg.Reuse.Epsilon != nil ||
		cfg.Reuse.Gamma != nil ||
		cfg.Reuse.NStep != nil ||
		strings.TrimSpace(cfg.Reuse.ModelSavePath) != "" ||
		cfg.Reuse.EnableModelPersistence != nil ||
		cfg.Reuse.ResetModelOnStart != nil {
		b.WriteString("  reuse: {\n")
		if cfg.Reuse.Epsilon != nil {
			b.WriteString(fmt.Sprintf("    epsilon: %s,\n", strconv.FormatFloat(*cfg.Reuse.Epsilon, 'f', -1, 64)))
		}
		if cfg.Reuse.Gamma != nil {
			b.WriteString(fmt.Sprintf("    gamma: %s,\n", strconv.FormatFloat(*cfg.Reuse.Gamma, 'f', -1, 64)))
		}
		if cfg.Reuse.NStep != nil {
			b.WriteString(fmt.Sprintf("    n_step: %d,\n", *cfg.Reuse.NStep))
		}
		if modelSavePath := strings.TrimSpace(cfg.Reuse.ModelSavePath); modelSavePath != "" {
			b.WriteString(fmt.Sprintf("    model_save_path: %q,\n", modelSavePath))
		}
		if cfg.Reuse.EnableModelPersistence != nil {
			b.WriteString(fmt.Sprintf("    enable_model_persistence: %t,\n", *cfg.Reuse.EnableModelPersistence))
		}
		if cfg.Reuse.ResetModelOnStart != nil {
			b.WriteString(fmt.Sprintf("    reset_model_on_start: %t,\n", *cfg.Reuse.ResetModelOnStart))
		}
		b.WriteString("  },\n")
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
