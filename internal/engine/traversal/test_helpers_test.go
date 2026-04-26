package traversal_test

import (
	enginestate "trek/internal/engine/state"
)

// testTraversalContext 创建一个用于测试的 TraversalContext 快照。
func testTraversalContext() enginestate.TraversalContext {
	return enginestate.TraversalContext{
		Step:          1,
		Mode:          "Explore",
		PageName:      "TestActivity",
		PageSignature: "sig_001",
	}
}