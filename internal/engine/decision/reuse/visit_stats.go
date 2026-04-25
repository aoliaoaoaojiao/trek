package reuse

import (
	"sync"
	"trek/internal/engine/decision/shared/types"
)

type reuseVisitStats struct {
	totalVisits  int64
	visitedPages map[string]struct{}
	lock         sync.RWMutex
}

func (s *reuseVisitStats) Record(state types.IState) {
	if state == nil {
		return
	}
	pageName := state.GetPageNameString()
	s.lock.Lock()
	s.totalVisits++
	if pageName != "" {
		s.visitedPages[pageName] = struct{}{}
	}
	s.lock.Unlock()
}

func (s *reuseVisitStats) Total() int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.totalVisits
}

func (s *reuseVisitStats) SnapshotPages() map[string]struct{} {
	s.lock.RLock()
	defer s.lock.RUnlock()
	pages := make(map[string]struct{}, len(s.visitedPages))
	for page := range s.visitedPages {
		pages[page] = struct{}{}
	}
	return pages
}
