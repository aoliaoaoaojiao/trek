package runtime

func LoadConfigFile(resourceMappingFilepath string) error {
	return defaultRuntime.LoadConfigFile(resourceMappingFilepath)
}

func LoadPluginsFromConfig(configPath string) error {
	return defaultRuntime.LoadPluginsFromConfig(configPath)
}

// LoadResourceMapping 加载资源配置（主入口）。
func LoadResourceMapping(resourceMappingFilepath string) {
	_ = LoadConfigFile(resourceMappingFilepath)
}

// Deprecated: 请使用 LoadResourceMapping。
func LoadResMapping(resMappingFilepath string) {
	LoadResourceMapping(resMappingFilepath)
}

func CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {
	return defaultRuntime.CheckPointIsInBlackRects(activity, pointX, pointY)
}
