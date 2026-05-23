package monkey

import (
	"context"
	"strings"
)

const (
	// PageNameStrategyActivityOnly 始终只使用当前 Activity，失败时返回 UnknownPage。
	PageNameStrategyActivityOnly = "activity_only"
	// PageNameStrategyStructureFingerprint 显式使用页面树结构指纹解析页面名。
	PageNameStrategyStructureFingerprint = "structure_fingerprint"
	// PageNameStrategyImageFingerprint 基于截图生成页面指纹，适用于 XML dump 不可用的场景。
	PageNameStrategyImageFingerprint = "image_fingerprint"
)

func resolveBasePageNameByStrategy(ctx context.Context, r *Runner, xml string, screenshot []byte) string {
	strategy := normalizePageNameStrategy(r.cfg.PageNameStrategy, r.cfg.PageSourceType)
	switch strategy {
	case PageNameStrategyImageFingerprint:
		if r.cfg.ImageSignatureFunc != nil {
			if pageName := resolveImageFingerprintPageName(screenshot, r.cfg.ImageSignatureFunc, r.cfg.ImageFingerprintRegions); pageName != "" {
				return pageName
			}
		}
		if r.fuzzyMatcher != nil && r.fuzzyMatcher.threshold > 0 {
			if pageName := r.fuzzyMatcher.Resolve(screenshot, r.cfg.ImageFingerprintRegions); pageName != "" {
				return pageName
			}
		} else if pageName := resolveImageFingerprintPageName(screenshot, nil, r.cfg.ImageFingerprintRegions); pageName != "" {
			return pageName
		}
		if r.cfg.PageNameResolverEx != nil {
			return r.cfg.PageNameResolverEx.ResolvePageName(xml, screenshot)
		}
		return resolveFallbackPageName(xml, r.cfg.PageNameResolver)
	case PageNameStrategyStructureFingerprint:
		return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, "")
	case PageNameStrategyActivityOnly:
		if activity, ok := resolveCurrentActivityName(ctx, r.driver); ok {
			return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, activity)
		}
		return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, "")
	default:
		return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, "")
	}
}

// ResolvePageNameByStrategy 使用 monkey 运行时同款策略解析页面名，供 web 预览等外部调试入口复用。
func ResolvePageNameByStrategy(xml string, resolver PageNameResolver, strategy string, pageSourceType string, currentActivity string) string {
	normalized := normalizePageNameStrategy(strategy, pageSourceType)
	activity := strings.TrimSpace(currentActivity)
	switch normalized {
	case PageNameStrategyStructureFingerprint:
		return ResolvePageName(xml, resolver)
	case PageNameStrategyActivityOnly:
		if activity != "" {
			return activity
		}
		return "UnknownPage"
	case PageNameStrategyImageFingerprint:
		// image_fingerprint 需要截图，外部调用方无法提供，fallback 到 XML 解析。
		return ResolvePageName(xml, resolver)
	default:
		return ResolvePageName(xml, resolver)
	}
}

func normalizePageNameStrategy(strategy string, pageSourceType string) string {
	text := strings.ToLower(strings.TrimSpace(strategy))
	if text == "" || text == "auto" {
		return PageNameStrategyStructureFingerprint
	}
	switch text {
	case PageNameStrategyStructureFingerprint:
		return PageNameStrategyStructureFingerprint
	case PageNameStrategyActivityOnly:
		return PageNameStrategyActivityOnly
	case PageNameStrategyImageFingerprint:
		return PageNameStrategyImageFingerprint
	default:
		// 未知策略按 auto 处理，确保兼容。
		return PageNameStrategyStructureFingerprint
	}
}

func resolveCurrentActivityName(ctx context.Context, driver any) (string, bool) {
	provider, ok := driver.(currentActivityProvider)
	if !ok || provider == nil {
		return "", false
	}
	activity, err := provider.GetCurrentActivity(ctx)
	if err != nil {
		return "", false
	}
	activity = strings.TrimSpace(activity)
	if activity == "" {
		return "", false
	}
	return activity, true
}

// resolveFallbackPageName 在 PageNameResolverEx 不可用时 fallback 到 XML 解析。
func resolveFallbackPageName(xml string, resolver PageNameResolver) string {
	if resolver != nil {
		return resolver(xml)
	}
	return defaultPageNameResolver(xml)
}
