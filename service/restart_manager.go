package service

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/logger"
)

const restartSignalDelay = 3 * time.Second

var defaultRestartManager = newRestartManager(restartSignalDelay, signalCurrentProcess)

type restartManager struct {
	mu           sync.Mutex
	inFlight     bool
	pendingTimer *time.Timer
	signalDelay  time.Duration
	signal       func() error
}

func init() {
	database.SetSendSighupHook(func() error {
		return defaultRestartManager.sendSighup()
	})
}

func newRestartManager(signalDelay time.Duration, signal func() error) *restartManager {
	return &restartManager{
		signalDelay: signalDelay,
		signal:      signal,
	}
}

func StopRestartManager() {
	defaultRestartManager.cancelPending()
}

func (m *restartManager) run(operation func() error) error {
	if !m.begin() {
		return nil
	}
	defer m.end()
	return operation()
}

func (m *restartManager) sendSighup() error {
	if !m.begin() {
		return nil
	}

	var timer *time.Timer
	timer = time.AfterFunc(m.signalDelay, func() {
		defer m.endPending(timer)
		if err := m.signal(); err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	})

	m.mu.Lock()
	m.pendingTimer = timer
	m.mu.Unlock()
	return nil
}

func (m *restartManager) cancelPending() {
	m.mu.Lock()
	timer := m.pendingTimer
	if timer == nil {
		m.mu.Unlock()
		return
	}
	m.pendingTimer = nil
	if timer.Stop() {
		m.inFlight = false
	}
	m.mu.Unlock()
}

func (m *restartManager) begin() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inFlight {
		return false
	}
	m.inFlight = true
	return true
}

func (m *restartManager) end() {
	m.mu.Lock()
	m.inFlight = false
	m.mu.Unlock()
}

func (m *restartManager) endPending(timer *time.Timer) {
	m.mu.Lock()
	if m.pendingTimer == timer {
		m.pendingTimer = nil
	}
	m.inFlight = false
	m.mu.Unlock()
}

func signalCurrentProcess() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	return process.Signal(syscall.SIGHUP)
}
