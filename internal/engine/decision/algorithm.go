package decision

import "fmt"

// AlgorithmType 表示决策算法类型。
type AlgorithmType int

const (
	AlgorithmRandom    AlgorithmType = iota
	AlgorithmReuse     AlgorithmType = 4
	AlgorithmUctBandit AlgorithmType = 7
)

var algorithmTypeName = map[AlgorithmType]string{
	AlgorithmRandom:    "Random",
	AlgorithmReuse:     "reuse",
	AlgorithmUctBandit: "uctbandit",
}

func (at AlgorithmType) String() string {
	if name, ok := algorithmTypeName[at]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", at)
}

