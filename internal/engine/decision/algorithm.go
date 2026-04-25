package decision

import "fmt"

// AlgorithmType 表示决策算法类型。
type AlgorithmType int

const (
	AlgorithmRandom AlgorithmType = iota
	AlgorithmReuse  AlgorithmType = 4
	AlgorithmServer AlgorithmType = 6
)

var algorithmTypeName = map[AlgorithmType]string{
	AlgorithmRandom: "Random",
	AlgorithmReuse:  "reuse",
	AlgorithmServer: "Server",
}

func (at AlgorithmType) String() string {
	if name, ok := algorithmTypeName[at]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", at)
}

