package graph

import (
	"strings"
	"testing"

	"trek/internal/engine/core/types"
)

func TestAddAgentRejectsUnknownAlgorithm(t *testing.T) {
	model := NewModel("com.demo.app")

	agent, err := model.AddAgent(DefaultDeviceID, "missing_algorithm", types.Phone)
	if err == nil {
		t.Fatalf("未知算法应返回错误")
	}
	if agent != nil {
		t.Fatalf("未知算法不应返回 agent")
	}
	if !strings.Contains(err.Error(), "未注册的决策算法") {
		t.Fatalf("错误信息不符合预期: %v", err)
	}
	if model.AgentSize() != 0 {
		t.Fatalf("失败场景不应写入 agent map，实际 AgentSize=%d", model.AgentSize())
	}
}
