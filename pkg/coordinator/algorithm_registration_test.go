package coordinator

import (
	"testing"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
)

func TestNewRegistersSupportedAlgorithms(t *testing.T) {
	algorithms := []decision.AlgorithmType{
		decision.AlgorithmReuse,
		decision.AlgorithmUctBandit,
		decision.AlgorithmRandom,
	}

	for _, algorithm := range algorithms {
		t.Run(algorithm.String(), func(t *testing.T) {
			coord, err := New(Config{
				PackageName: "com.demo.app",
				Algorithm:   algorithm,
				DeviceType:  types.Phone,
			})
			if err != nil {
				t.Fatalf("创建协调器失败: %v", err)
			}

			model := coord.runtime.GetModel()
			if model == nil {
				t.Fatalf("runtime model 不应为空")
			}

			agent := model.GetAgent(decision.DefaultDeviceID)
			if agent == nil {
				t.Fatalf("算法 %s 未注册成功，agent 为空", algorithm.String())
			}
		})
	}
}
