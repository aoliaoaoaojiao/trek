package monkey

import (
	"context"
	"strings"
	"sync"
	"time"
	"trek/pkg/driver/common"
)

type currentPackageProvider interface {
	GetCurrentPackage(ctx context.Context) (string, error)
}

type foregroundPackageMonitor struct {
	provider currentPackageProvider
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	pkg      string
	err      error
	updated  bool
}

type healthSignalMonitor struct {
	driver      common.IDriver
	packageName string
	interval    time.Duration
	stopCh      chan struct{}
	doneCh      chan struct{}
	mu          sync.RWMutex
	crash       bool
	anr         bool
	updated     bool
}

func newHealthSignalMonitor(driver common.IDriver, packageName string, interval time.Duration) *healthSignalMonitor {
	return &healthSignalMonitor{
		driver:      driver,
		packageName: packageName,
		interval:    interval,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

func (m *healthSignalMonitor) start() {
	m.refresh()
	go func() {
		ticker := time.NewTicker(m.interval)
		defer func() {
			ticker.Stop()
			close(m.doneCh)
		}()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.refresh()
			}
		}
	}()
}

func (m *healthSignalMonitor) stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *healthSignalMonitor) refresh() {
	crash, crashErr := m.driver.CheckCrash(m.packageName)
	anr, anrErr := m.driver.CheckANR(m.packageName)
	m.mu.Lock()
	if crashErr == nil {
		m.crash = crash
	}
	if anrErr == nil {
		m.anr = anr
	}
	m.updated = true
	m.mu.Unlock()
}

func (m *healthSignalMonitor) snapshot() (crash bool, anr bool, updated bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.crash, m.anr, m.updated
}

func newForegroundPackageMonitor(provider currentPackageProvider, interval time.Duration) *foregroundPackageMonitor {
	return &foregroundPackageMonitor{
		provider: provider,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (m *foregroundPackageMonitor) start() {
	m.refresh()
	go func() {
		ticker := time.NewTicker(m.interval)
		defer func() {
			ticker.Stop()
			close(m.doneCh)
		}()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.refresh()
			}
		}
	}()
}

func (m *foregroundPackageMonitor) stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *foregroundPackageMonitor) refresh() {
	pkg, err := m.provider.GetCurrentPackage(context.Background())
	m.mu.Lock()
	m.pkg = strings.TrimSpace(pkg)
	m.err = err
	m.updated = true
	m.mu.Unlock()
}

func (m *foregroundPackageMonitor) snapshot() (string, error, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pkg, m.err, m.updated
}

func (m *foregroundPackageMonitor) setCurrentPackage(pkg string) {
	m.mu.Lock()
	m.pkg = strings.TrimSpace(pkg)
	m.err = nil
	m.updated = true
	m.mu.Unlock()
}

func (r *Runner) startForegroundPackageMonitor() {
	targetPackage := strings.TrimSpace(r.cfg.PackageName)
	if targetPackage == "" {
		return
	}
	provider, ok := r.driver.(currentPackageProvider)
	if !ok || provider == nil {
		return
	}
	monitor := newForegroundPackageMonitor(provider, r.cfg.ForegroundMonitorInterval)
	monitor.start()
	r.monitor = monitor
}

func (r *Runner) stopForegroundPackageMonitor() {
	if r.monitor == nil {
		return
	}
	r.monitor.stop()
	r.monitor = nil
}

func (r *Runner) startHealthSignalMonitor() {
	if !r.cfg.StopOnCrash && !r.cfg.StopOnANR {
		return
	}
	targetPackage := strings.TrimSpace(r.cfg.PackageName)
	monitor := newHealthSignalMonitor(r.driver, targetPackage, r.cfg.HealthSignalMonitorInterval)
	monitor.start()
	r.healthMonitor = monitor
}

func (r *Runner) stopHealthSignalMonitor() {
	if r.healthMonitor == nil {
		return
	}
	r.healthMonitor.stop()
	r.healthMonitor = nil
}

func (r *Runner) snapshotHealthSignals() (crash bool, anr bool, ready bool) {
	if r.healthMonitor == nil {
		return false, false, false
	}
	return r.healthMonitor.snapshot()
}
