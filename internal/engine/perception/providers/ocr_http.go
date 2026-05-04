package providers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

const (
	defaultOCRTimeout             = 10 * time.Second
	defaultOCRCandidateConfidence = 0.6
	defaultOCRMaxCandidates       = 24
	requestFormatDefault          = "default"
	requestFormatAistudioOCR      = "aistudio_ocr"
	requestFormatAistudioLayout   = "aistudio_layout"
)

// OCRHTTPProviderConfig 定义截图 OCR 候选提供器配置。
type OCRHTTPProviderConfig struct {
	Endpoint      string
	APIKey        string
	Timeout       time.Duration
	Headers       map[string]string
	MaxCandidates int
	RequestFormat string
}

// OCRHTTPProvider 通过外部 OCR HTTP 服务将截图文本框转换为点击候选。
type OCRHTTPProvider struct {
	endpoint      string
	apiKey        string
	client        *http.Client
	headers       map[string]string
	maxCandidates int
	requestFormat string
}

// NewOCRHTTPProvider 创建 OCR HTTP 提供器。
func NewOCRHTTPProvider(cfg OCRHTTPProviderConfig) (*OCRHTTPProvider, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("ocr endpoint 不能为空")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultOCRTimeout
	}
	headers := make(map[string]string, len(cfg.Headers))
	for key, value := range cfg.Headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		headers[key] = value
	}
	return &OCRHTTPProvider{
		endpoint:      endpoint,
		apiKey:        strings.TrimSpace(cfg.APIKey),
		client:        &http.Client{Timeout: timeout},
		headers:       headers,
		maxCandidates: resolveOCRMaxCandidates(cfg.MaxCandidates),
		requestFormat: resolveOCRRequestFormat(endpoint, cfg.RequestFormat),
	}, nil
}

// BuildCandidates 将 OCR 文本区域直接映射为 CLICK 候选。
func (p *OCRHTTPProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if p == nil || len(ctx.Screenshot) == 0 {
		return nil, nil
	}
	width, height := decodeImageSize(ctx.Screenshot)
	data, err := p.buildPayload(ctx, width, height)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, p.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	p.setAuthHeader(req)
	for key, value := range p.headers {
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ocr endpoint 响应异常: status=%d body=%s", resp.StatusCode, truncateText(string(body), 512))
	}

	var output ocrResponse
	if err := json.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("解析 ocr 响应失败: %w", err)
	}
	items := p.toCandidates(output, width, height)
	if len(items) > 0 {
		return items, nil
	}
	// 兼容 aistudio /ocr、layout-parsing 等多层结构响应。
	fallback := extractOCRRegionsFromArbitraryJSON(body)
	return p.toCandidates(ocrResponse{Regions: fallback}, width, height), nil
}

func (p *OCRHTTPProvider) toCandidates(output ocrResponse, width int, height int) []perception.Candidate {
	regions := output.Regions
	if len(regions) == 0 {
		regions = output.Results
	}
	if len(regions) == 0 {
		return nil
	}

	limit := len(regions)
	if p != nil && p.maxCandidates > 0 && limit > p.maxCandidates {
		limit = p.maxCandidates
	}

	items := make([]perception.Candidate, 0, limit)
	for _, region := range regions[:limit] {
		bounds, ok := normalizeOCRBounds(region.Bounds, width, height)
		if !ok {
			continue
		}
		cmd := types.NewActionCommand()
		cmd.Act = types.CLICK
		cmd.Pos = *types.NewRect(bounds[0], bounds[1], bounds[2], bounds[3])

		text := strings.TrimSpace(region.Text)
		intent := "ocr_region_click"
		if text != "" {
			intent = "ocr_click:" + text
		}
		confidence := region.Confidence
		if confidence <= 0 {
			confidence = defaultOCRCandidateConfidence
		}
		if confidence > 1 {
			confidence = 1
		}

		item := perception.NewCandidate(cmd, perception.SourceOCR, intent, map[string]string{
			"provider": "ocr_http",
			"ocr_text": text,
		})
		item.Confidence = confidence
		items = append(items, item)
	}
	return items
}

func (p *OCRHTTPProvider) buildPayload(ctx enginestate.TraversalContext, width int, height int) ([]byte, error) {
	if p != nil && (p.requestFormat == requestFormatAistudioLayout || p.requestFormat == requestFormatAistudioOCR) {
		payload := map[string]any{
			"file":                      base64.StdEncoding.EncodeToString(ctx.Screenshot),
			"fileType":                  1,
			"useDocPreprocessor":        false,
			"useDocOrientationClassify": false,
			"useDocUnwarping":           false,
		}
		if p.requestFormat == requestFormatAistudioLayout {
			payload["useChartRecognition"] = false
		}
		return json.Marshal(payload)
	}
	payload := ocrRequest{
		PageName:         strings.TrimSpace(ctx.PageName),
		ScreenshotBase64: base64.StdEncoding.EncodeToString(ctx.Screenshot),
		ImageWidth:       width,
		ImageHeight:      height,
	}
	return json.Marshal(payload)
}

func (p *OCRHTTPProvider) setAuthHeader(req *http.Request) {
	if p == nil || req == nil || p.apiKey == "" {
		return
	}
	if p.requestFormat == requestFormatAistudioLayout || p.requestFormat == requestFormatAistudioOCR {
		req.Header.Set("Authorization", "token "+p.apiKey)
		return
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func resolveOCRMaxCandidates(value int) int {
	if value <= 0 {
		return defaultOCRMaxCandidates
	}
	return value
}

func resolveOCRRequestFormat(endpoint string, configured string) string {
	mode := strings.ToLower(strings.TrimSpace(configured))
	if mode == requestFormatAistudioLayout || mode == requestFormatAistudioOCR || mode == requestFormatDefault {
		return mode
	}
	lowEndpoint := strings.ToLower(strings.TrimSpace(endpoint))
	if strings.Contains(lowEndpoint, "/layout-parsing") {
		return requestFormatAistudioLayout
	}
	if strings.HasSuffix(lowEndpoint, "/ocr") || strings.Contains(lowEndpoint, "/ocr?") {
		return requestFormatAistudioOCR
	}
	return requestFormatDefault
}

func decodeImageSize(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func extractOCRRegionsFromArbitraryJSON(data []byte) []ocrRegion {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make([]ocrRegion, 0, 16)
	walkOCRNode(raw, &result)
	return result
}

func walkOCRNode(node any, acc *[]ocrRegion) {
	switch typed := node.(type) {
	case map[string]any:
		if regions := extractRegionsFromMap(typed); len(regions) > 0 {
			*acc = append(*acc, regions...)
		}
		for _, value := range typed {
			walkOCRNode(value, acc)
		}
	case []any:
		for _, value := range typed {
			walkOCRNode(value, acc)
		}
	}
}

func extractRegionsFromMap(item map[string]any) []ocrRegion {
	regions := make([]ocrRegion, 0, 4)
	if ocrResults := extractAistudioOCRResults(item); len(ocrResults) > 0 {
		regions = append(regions, ocrResults...)
	}
	if region, ok := extractRegionFromMap(item); ok {
		regions = append(regions, region)
	}
	if tableRegions := extractTableCellRegions(item); len(tableRegions) > 0 {
		regions = append(regions, tableRegions...)
	}
	return regions
}

func extractAistudioOCRResults(item map[string]any) []ocrRegion {
	textsValue, ok := item["rec_texts"]
	if !ok {
		return nil
	}
	boxesValue, ok := item["rec_boxes"]
	if !ok {
		return nil
	}
	texts := readStringArray(textsValue)
	boxes := readBoundsArray(boxesValue)
	if len(texts) == 0 || len(boxes) == 0 {
		return nil
	}

	scores := readFloatArrayFromAny(item["rec_scores"])
	limit := len(texts)
	if len(boxes) < limit {
		limit = len(boxes)
	}
	regions := make([]ocrRegion, 0, limit)
	for i := 0; i < limit; i++ {
		text := strings.TrimSpace(texts[i])
		if text == "" || len(boxes[i]) != 4 {
			continue
		}
		confidence := defaultOCRCandidateConfidence
		if i < len(scores) && scores[i] > 0 {
			confidence = scores[i]
		}
		regions = append(regions, ocrRegion{
			Text:       text,
			Confidence: confidence,
			Bounds:     boxes[i],
		})
	}
	return regions
}

func extractRegionFromMap(item map[string]any) (ocrRegion, bool) {
	var region ocrRegion
	text := readStringByKeys(item, "text", "content", "ocr_text", "recognizedText")
	confidence := readFloatByKeys(item, "confidence", "score", "probability")
	bounds, ok := readBoundsByKeys(item, "bounds", "bbox", "box", "rect")
	if !ok {
		bounds, ok = readPolygonByKeys(item, "polygon", "points", "quad")
	}
	if !ok {
		return region, false
	}
	region.Text = strings.TrimSpace(text)
	region.Confidence = confidence
	region.Bounds = bounds
	return region, true
}

var (
	htmlTableRowPattern  = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	htmlTableCellPattern = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)
	htmlTagPattern       = regexp.MustCompile(`(?is)<[^>]+>`)
)

func extractTableCellRegions(item map[string]any) []ocrRegion {
	content := strings.TrimSpace(readStringByKeys(item, "block_content"))
	if content == "" || !strings.Contains(strings.ToLower(content), "<table") {
		return nil
	}
	bounds, ok := readBoundsByKeys(item, "block_bbox", "coordinate", "bounds", "bbox")
	if !ok || len(bounds) != 4 {
		return nil
	}

	rows := parseHTMLTableRows(content)
	if len(rows) == 0 {
		return nil
	}
	colCount := maxTableColumnCount(rows)
	if colCount <= 0 {
		return nil
	}

	rowCount := len(rows)
	rowHeight := (bounds[3] - bounds[1]) / float64(rowCount)
	colWidth := (bounds[2] - bounds[0]) / float64(colCount)
	if rowHeight <= 0 || colWidth <= 0 {
		return nil
	}

	regions := make([]ocrRegion, 0, rowCount)
	for rowIndex, row := range rows {
		for colIndex, cellText := range row {
			cellText = normalizeOCRTableText(cellText)
			if cellText == "" {
				continue
			}
			left := bounds[0] + float64(colIndex)*colWidth
			top := bounds[1] + float64(rowIndex)*rowHeight
			right := left + colWidth
			bottom := top + rowHeight
			regions = append(regions, ocrRegion{
				Text:       cellText,
				Confidence: readFloatByKeys(item, "score", "confidence", "probability"),
				Bounds:     []float64{left, top, right, bottom},
			})
		}
	}
	return regions
}

func parseHTMLTableRows(content string) [][]string {
	rowMatches := htmlTableRowPattern.FindAllStringSubmatch(content, -1)
	if len(rowMatches) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(rowMatches))
	for _, rowMatch := range rowMatches {
		if len(rowMatch) < 2 {
			continue
		}
		cellMatches := htmlTableCellPattern.FindAllStringSubmatch(rowMatch[1], -1)
		if len(cellMatches) == 0 {
			continue
		}
		row := make([]string, 0, len(cellMatches))
		for _, cellMatch := range cellMatches {
			if len(cellMatch) < 2 {
				row = append(row, "")
				continue
			}
			row = append(row, normalizeOCRTableText(cellMatch[1]))
		}
		rows = append(rows, row)
	}
	return rows
}

func normalizeOCRTableText(text string) string {
	text = html.UnescapeString(text)
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func maxTableColumnCount(rows [][]string) int {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

func readStringByKeys(item map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func readFloatByKeys(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if num, ok := toFloat(value); ok {
			return num
		}
	}
	return 0
}

func readStringArray(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			result = append(result, "")
			continue
		}
		result = append(result, text)
	}
	return result
}

func readBoundsArray(value any) [][]float64 {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([][]float64, 0, len(raw))
	for _, item := range raw {
		if bounds, ok := toBounds(item); ok {
			result = append(result, bounds)
			continue
		}
		if values := readFloatArray(item); len(values) == 4 {
			result = append(result, values)
		}
	}
	return result
}

func readFloatArrayFromAny(value any) []float64 {
	return readFloatArray(value)
}

func readBoundsByKeys(item map[string]any, keys ...string) ([]float64, bool) {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if bounds, ok := toBounds(value); ok {
			return bounds, true
		}
	}
	return nil, false
}

func readPolygonByKeys(item map[string]any, keys ...string) ([]float64, bool) {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		values := readFloatArray(value)
		if len(values) < 8 {
			continue
		}
		left, top, right, bottom := values[0], values[1], values[0], values[1]
		for i := 0; i+1 < len(values); i += 2 {
			x, y := values[i], values[i+1]
			if x < left {
				left = x
			}
			if y < top {
				top = y
			}
			if x > right {
				right = x
			}
			if y > bottom {
				bottom = y
			}
		}
		return []float64{left, top, right, bottom}, true
	}
	return nil, false
}

func toBounds(value any) ([]float64, bool) {
	switch typed := value.(type) {
	case []any:
		values := readFloatArray(typed)
		if len(values) == 4 {
			return values, true
		}
	case map[string]any:
		left, lok := pickFirstFloat(typed, "left", "x1", "x")
		top, tok := pickFirstFloat(typed, "top", "y1", "y")
		right, rok := pickFirstFloat(typed, "right", "x2")
		bottom, bok := pickFirstFloat(typed, "bottom", "y2")
		if lok && tok && rok && bok {
			return []float64{left, top, right, bottom}, true
		}
		width, wok := pickFirstFloat(typed, "width", "w")
		height, hok := pickFirstFloat(typed, "height", "h")
		if lok && tok && wok && hok {
			return []float64{left, top, left + width, top + height}, true
		}
	}
	return nil, false
}

func readFloatArray(value any) []float64 {
	switch typed := value.(type) {
	case []any:
		result := make([]float64, 0, len(typed))
		for _, item := range typed {
			if num, ok := toFloat(item); ok {
				result = append(result, num)
				continue
			}
			if pair, ok := item.([]any); ok && len(pair) >= 2 {
				x, xok := toFloat(pair[0])
				y, yok := toFloat(pair[1])
				if xok && yok {
					result = append(result, x, y)
				}
				continue
			}
			if point, ok := item.(map[string]any); ok {
				x, xok := pickFirstFloat(point, "x", "X")
				y, yok := pickFirstFloat(point, "y", "Y")
				if xok && yok {
					result = append(result, x, y)
				}
			}
		}
		return result
	default:
		return nil
	}
}

func pickFirstFloat(item map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if num, ok := toFloat(value); ok {
			return num, true
		}
	}
	return 0, false
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		num, err := typed.Float64()
		return num, err == nil
	}
	return 0, false
}

func normalizeOCRBounds(bounds []float64, width int, height int) ([4]float64, bool) {
	var zero [4]float64
	if len(bounds) != 4 {
		return zero, false
	}
	left, top, right, bottom := bounds[0], bounds[1], bounds[2], bounds[3]
	if right <= left || bottom <= top {
		return zero, false
	}
	if exceedsUnitBounds(bounds) {
		if width <= 0 || height <= 0 {
			return zero, false
		}
		left /= float64(width)
		right /= float64(width)
		top /= float64(height)
		bottom /= float64(height)
	}

	left = clamp01(left)
	top = clamp01(top)
	right = clamp01(right)
	bottom = clamp01(bottom)
	if right <= left || bottom <= top {
		return zero, false
	}
	return [4]float64{left, top, right, bottom}, true
}

func exceedsUnitBounds(bounds []float64) bool {
	for _, value := range bounds {
		if value > 1 {
			return true
		}
	}
	return false
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func truncateText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

type ocrRequest struct {
	PageName         string `json:"page_name,omitempty"`
	ScreenshotBase64 string `json:"screenshot_base64"`
	ImageWidth       int    `json:"image_width,omitempty"`
	ImageHeight      int    `json:"image_height,omitempty"`
}

type ocrResponse struct {
	Regions []ocrRegion `json:"regions"`
	Results []ocrRegion `json:"results"`
}

type ocrRegion struct {
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Bounds     []float64 `json:"bounds"`
}
