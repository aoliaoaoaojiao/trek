package monkey

import (
	"context"
	"strings"
)

const (
	// PageNameStrategyUIAActivityFirst 在 UIA 页面源下优先使用当前 Activity，失败时回退 XML 解析。
	PageNameStrategyUIAActivityFirst = "uia_activity_first"
	// PageNameStrategyXMLOnly 始终只使用 XML 结构指纹解析页面名。
	PageNameStrategyXMLOnly = "xml_only"
	// PageNameStrategyActivityOnly 始终只使用当前 Activity，失败时返回 UnknownPage。
	PageNameStrategyActivityOnly = "activity_only"
	// PageNameStrategyXMLFingerprint 显式使用 XML 结构指纹解析页面名。
	PageNameStrategyXMLFingerprint = "xml_fingerprint"
	// PageNameStrategyStructureFingerprint 显式使用页面树结构指纹解析页面名。
	PageNameStrategyStructureFingerprint = "structure_fingerprint"
)

func resolveBasePageNameByStrategy(ctx context.Context, r *Runner, xml string) string {
	strategy := normalizePageNameStrategy(r.cfg.PageNameStrategy, r.cfg.PageSourceType)
	switch strategy {
	case PageNameStrategyXMLOnly, PageNameStrategyXMLFingerprint, PageNameStrategyStructureFingerprint:
		return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, "")
	case PageNameStrategyActivityOnly:
		if activity, ok := resolveCurrentActivityName(ctx, r.driver); ok {
			return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, activity)
		}
		return ResolvePageNameByStrategy(xml, r.cfg.PageNameResolver, strategy, r.cfg.PageSourceType, "")
	case PageNameStrategyUIAActivityFirst:
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
	case PageNameStrategyXMLOnly, PageNameStrategyXMLFingerprint, PageNameStrategyStructureFingerprint:
		return ResolvePageName(xml, resolver)
	case PageNameStrategyActivityOnly:
		if activity != "" {
			return activity
		}
		return "UnknownPage"
	case PageNameStrategyUIAActivityFirst:
		if activity != "" {
			return activity
		}
		return ResolvePageName(xml, resolver)
	default:
		return ResolvePageName(xml, resolver)
	}
}

func normalizePageNameStrategy(strategy string, pageSourceType string) string {
	text := strings.ToLower(strings.TrimSpace(strategy))
	if text == "" || text == "auto" {
		if strings.EqualFold(strings.TrimSpace(pageSourceType), string(defaultPageSourceType)) {
			return PageNameStrategyUIAActivityFirst
		}
		return PageNameStrategyXMLOnly
	}
	switch text {
	case PageNameStrategyUIAActivityFirst:
		return PageNameStrategyUIAActivityFirst
	case PageNameStrategyXMLOnly:
		return PageNameStrategyXMLOnly
	case PageNameStrategyXMLFingerprint:
		return PageNameStrategyXMLFingerprint
	case PageNameStrategyStructureFingerprint:
		return PageNameStrategyStructureFingerprint
	case PageNameStrategyActivityOnly:
		return PageNameStrategyActivityOnly
	default:
		// 未知策略按 auto 处理，确保兼容。
		if strings.EqualFold(strings.TrimSpace(pageSourceType), string(defaultPageSourceType)) {
			return PageNameStrategyUIAActivityFirst
		}
		return PageNameStrategyXMLOnly
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
