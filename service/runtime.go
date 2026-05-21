package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/deposist/s-ui-rus-inst/core"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

const defaultCoreStartCooldown = 15 * time.Second

type CoreProvider interface {
	Core() *core.Core
}

type CoreProviderFunc func() *core.Core

func (f CoreProviderFunc) Core() *core.Core {
	if f == nil {
		return nil
	}
	return f()
}

type LastUpdateStore struct {
	value atomic.Int64
}

func NewLastUpdateStore() *LastUpdateStore {
	return &LastUpdateStore{}
}

func (s *LastUpdateStore) Set(value int64) {
	if s == nil {
		return
	}
	s.value.Store(value)
	LastUpdate = value
}

func (s *LastUpdateStore) Get() int64 {
	if s == nil {
		return 0
	}
	return s.value.Load()
}

type Runtime struct {
	mu sync.RWMutex

	coreProvider     CoreProvider
	restartManager   *restartManager
	lastUpdate       *LastUpdateStore
	auditWriter      *auditWriter
	telegramNotifier *telegramNotifier
	tokenUse         *tokenUseDebouncer

	coreStartCooldown time.Duration
	lastStartFailTime time.Time
}

func NewRuntime(coreInstance *core.Core) *Runtime {
	return NewRuntimeWithCoreProvider(CoreProviderFunc(func() *core.Core {
		return coreInstance
	}))
}

func NewRuntimeWithCoreProvider(provider CoreProvider) *Runtime {
	return &Runtime{
		coreProvider:      provider,
		restartManager:    newRestartManager(restartSignalDelay, signalCurrentProcess),
		lastUpdate:        NewLastUpdateStore(),
		auditWriter:       newAuditWriter(auditQueueCapacity, auditBatchSize, auditFlushInterval, writeAuditEvents),
		tokenUse:          newTokenUseDebouncer(tokenUseFlushInterval, flushTokenUseUpdates),
		coreStartCooldown: defaultCoreStartCooldown,
	}
}

func (r *Runtime) SetCore(coreInstance *core.Core) {
	if r == nil {
		return
	}
	r.SetCoreProvider(CoreProviderFunc(func() *core.Core {
		return coreInstance
	}))
}

func (r *Runtime) SetCoreProvider(provider CoreProvider) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.coreProvider = provider
	r.mu.Unlock()
}

func (r *Runtime) Core() *core.Core {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	provider := r.coreProvider
	r.mu.RUnlock()
	if provider == nil {
		return nil
	}
	return provider.Core()
}

func (r *Runtime) RestartScheduler() RestartScheduler {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	manager := r.restartManager
	r.mu.RUnlock()
	return manager
}

func (r *Runtime) restart() *restartManager {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	manager := r.restartManager
	r.mu.RUnlock()
	return manager
}

func (r *Runtime) updates() *LastUpdateStore {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	store := r.lastUpdate
	r.mu.RUnlock()
	return store
}

func (r *Runtime) audit() *auditWriter {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	writer := r.auditWriter
	r.mu.RUnlock()
	return writer
}

func (r *Runtime) replaceAuditWriterIfCurrent(current *auditWriter) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.auditWriter == current {
		r.auditWriter = newAuditWriter(auditQueueCapacity, auditBatchSize, auditFlushInterval, writeAuditEvents)
	}
	r.mu.Unlock()
}

func (r *Runtime) telegram() *telegramNotifier {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.telegramNotifier == nil {
		r.telegramNotifier = newDefaultTelegramNotifier()
	}
	notifier := r.telegramNotifier
	return notifier
}

func (r *Runtime) replaceTelegramNotifierIfCurrent(current *telegramNotifier) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.telegramNotifier == current {
		r.telegramNotifier = newDefaultTelegramNotifier()
	}
	r.mu.Unlock()
}

func (r *Runtime) tokenUseDebouncer() *tokenUseDebouncer {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	debouncer := r.tokenUse
	r.mu.RUnlock()
	return debouncer
}

func (r *Runtime) resetTokenUseDebouncer() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.tokenUse != nil {
		r.tokenUse.mu.Lock()
		r.tokenUse.epoch++
		if r.tokenUse.timer != nil {
			r.tokenUse.timer.Stop()
			r.tokenUse.timer = nil
		}
		r.tokenUse.mu.Unlock()
	}
	r.tokenUse = newTokenUseDebouncer(tokenUseFlushInterval, flushTokenUseUpdates)
	r.mu.Unlock()
}

func (r *Runtime) startCooldownActive() bool {
	if r == nil {
		return false
	}
	return time.Since(r.lastStartFailTime) < r.coreStartCooldown
}

func (r *Runtime) markCoreStartFailed() {
	if r == nil {
		return
	}
	r.lastStartFailTime = time.Now()
}

func (r *Runtime) markCoreStartSucceeded() {
	if r == nil {
		return
	}
	r.lastStartFailTime = time.Time{}
}

func (r *Runtime) coreStartCooldownDuration() time.Duration {
	if r == nil || r.coreStartCooldown <= 0 {
		return defaultCoreStartCooldown
	}
	return r.coreStartCooldown
}

var (
	defaultRuntimeMu sync.RWMutex
	defaultRuntime   = NewRuntimeWithCoreProvider(nil)
)

func DefaultRuntime() *Runtime {
	defaultRuntimeMu.RLock()
	runtime := defaultRuntime
	defaultRuntimeMu.RUnlock()
	return runtime
}

func SetDefaultRuntime(runtime *Runtime) {
	if runtime == nil {
		runtime = NewRuntimeWithCoreProvider(nil)
	}
	defaultRuntimeMu.Lock()
	defaultRuntime = runtime
	defaultRuntimeMu.Unlock()
}

func ReplaceDefaultRuntimeForTest(runtime *Runtime) func() {
	defaultRuntimeMu.Lock()
	previous := defaultRuntime
	if runtime == nil {
		runtime = NewRuntimeWithCoreProvider(nil)
	}
	defaultRuntime = runtime
	defaultRuntimeMu.Unlock()
	return func() {
		defaultRuntimeMu.Lock()
		defaultRuntime = previous
		defaultRuntimeMu.Unlock()
	}
}

func runtimeOrDefault(runtime *Runtime) *Runtime {
	if runtime != nil {
		return runtime
	}
	return DefaultRuntime()
}

func writeAuditRuntime(writer *auditWriter, event model.AuditEvent) {
	if writer == nil {
		return
	}
	writer.Enqueue(event)
}

// LastUpdate is kept as a compatibility mirror for older in-package tests and
// integrations. New code should use Runtime.lastUpdate via setLastUpdate and
// getLastUpdate instead.
//
// Deprecated: use the injected Runtime last-update store.
var LastUpdate int64
