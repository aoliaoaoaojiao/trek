package tool

import coretool "trek/internal/engine/core/tool"

// 以下为向后兼容的重新导出，实际实现已迁移至 core/tool。

var (
	HashString        = coretool.HashString
	HashInt           = coretool.HashInt
	IsZhCnByte        = coretool.IsZhCnByte
	IsZhCn            = coretool.IsZhCn
	RandomInt         = coretool.RandomInt
	RandomIntWithSeed = coretool.RandomIntWithSeed
	TrimString        = coretool.TrimString
	SplitString       = coretool.SplitString
	StringReplaceAll  = coretool.StringReplaceAll
	GetTimeFormatStr  = coretool.GetTimeFormatStr
	CurrentStamp      = coretool.CurrentStamp
	GetRandomChars    = coretool.GetRandomChars
)

const (
	AlphabetSeqLen   = coretool.AlphabetSeqLen
	AlphabetChSeqLen = coretool.AlphabetChSeqLen
)

var (
	AlphabetSeq   = coretool.AlphabetSeq
	AlphabetChSeq = coretool.AlphabetChSeq
)
