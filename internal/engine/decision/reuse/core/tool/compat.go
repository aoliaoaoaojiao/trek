package tool

import sharedtool "trek/internal/engine/decision/shared/tool"

func HashString(str string) uintptr {
	return sharedtool.HashString(str)
}

func HashInt(num int) uintptr {
	return sharedtool.HashInt(num)
}
