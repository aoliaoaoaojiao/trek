package types

import (
	"Trek/log"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
)

// Node 基础节点结构，用于统计访问状态和计数
type Node struct {
	VisitedCount int32
	ID           int32
}

// NewNode 创建新的节点
func NewNode() *Node {
	return &Node{
		VisitedCount: 0,
		ID:           0,
	}
}

// Visit 更新访问计数
func (n *Node) Visit(timestamp time.Time) {
	atomic.AddInt32(&n.VisitedCount, 1)
	log.Debugf("visit id:%s times %d", n.GetId(), n.VisitedCount)
}

// IsVisited 测试节点是否已被访问
func (n *Node) IsVisited() bool {
	return atomic.LoadInt32(&n.VisitedCount) > 0
}

// GetVisitedCount 获取访问计数
func (n *Node) GetVisitedCount() int32 {
	return atomic.LoadInt32(&n.VisitedCount)
}

// GetId 获取节点ID
func (n *Node) GetId() string {
	return strconv.Itoa(int(n.ID))
}

// GetIdi 获取节点ID(整数)
func (n *Node) GetIdi() int32 {
	return n.ID
}

// SetId 设置节点ID
func (n *Node) SetId(id int32) {
	n.ID = id
}

// String 实现Serializable接口
func (n *Node) String() string {
	return fmt.Sprintf("Node{id:%d, visited:%d}", n.ID, n.VisitedCount)
}

// PriorityNodeImpl 优先级节点实现
type PriorityNodeImpl struct {
	Priority int32
}

// NewPriorityNode 创建新的优先级节点
func NewPriorityNode() *PriorityNodeImpl {
	return &PriorityNodeImpl{
		Priority: 0,
	}
}

// GetPriority 获取优先级
func (p *PriorityNodeImpl) GetPriority() int32 {
	return atomic.LoadInt32(&p.Priority)
}

// SetPriority 设置优先级
func (p *PriorityNodeImpl) SetPriority(priority int32) {
	atomic.StoreInt32(&p.Priority, priority)
}

// Less 实现比较接口
func (p *PriorityNodeImpl) Less(other *PriorityNodeImpl) bool {
	return p.GetPriority() < other.GetPriority()
}
