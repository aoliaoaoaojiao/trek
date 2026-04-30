package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileLoadsPluginChainFromConfig(t *testing.T) {
	ResetModel()
	t.Cleanup(ResetModel)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.js")
	pluginDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("创建插件目录失败: %v", err)
	}

	pluginA := `const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_A",
      xml: ctx.page.xml + "<A/>"
    }
  }
};`
	pluginB := `const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_B",
      xml: ctx.page.xml + "<B/>"
    }
  }
};`
	config := `const config = {
  plugins: ["./plugins/a.plugin.js", "./plugins/b.plugin.js"]
};`

	if err := os.WriteFile(filepath.Join(pluginDir, "a.plugin.js"), []byte(pluginA), 0644); err != nil {
		t.Fatalf("写入 pluginA 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "b.plugin.js"), []byte(pluginB), 0644); err != nil {
		t.Fatalf("写入 pluginB 失败: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("写入 config 失败: %v", err)
	}

	if err := LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	pageName, xml, err := TransformPageInfoWithInput("Main", "<root/>", nil)
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 失败: %v", err)
	}
	if pageName != "Main_A_B" {
		t.Fatalf("插件链页面名不符合预期: %s", pageName)
	}
	if xml != "<root/><A/><B/>" {
		t.Fatalf("插件链 XML 不符合预期: %s", xml)
	}
}

func TestLoadConfigFileKeepsLegacySinglePlugin(t *testing.T) {
	ResetModel()
	t.Cleanup(ResetModel)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.js")
	content := `const config = {
  page_source: "uia"
};
const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_Legacy",
      xml: ctx.page.xml.replace("foo", "bar")
    }
  }
};`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	if err := LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	pageName, xml, err := TransformPageInfoWithInput("Main", "<node text=\"foo\"/>", nil)
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 失败: %v", err)
	}
	if pageName != "Main_Legacy" {
		t.Fatalf("旧单插件模式页面名不符合预期: %s", pageName)
	}
	if xml != "<node text=\"bar\"/>" {
		t.Fatalf("旧单插件模式 XML 不符合预期: %s", xml)
	}
}
