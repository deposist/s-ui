package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/logger"
)

const (
	tokenUseFlushInterval = time.Minute
	tokenUseBatchSize     = 100
)

type tokenUseUpdate struct {
	ip string
	ts int64
}

type tokenUseDebouncer struct {
	mu       sync.Mutex
	pending  map[uint]tokenUseUpdate
	timer    *time.Timer
	epoch    uint64
	interval time.Duration
	flush    func(map[uint]tokenUseUpdate) error
}

func newTokenUseDebouncer(interval time.Duration, flush func(map[uint]tokenUseUpdate) error) *tokenUseDebouncer {
	if interval <= 0 {
		interval = tokenUseFlushInterval
	}
	return &tokenUseDebouncer{
		pending:  make(map[uint]tokenUseUpdate),
		interval: interval,
		flush:    flush,
	}
}

func getTokenUseDebouncer() *tokenUseDebouncer {
	return DefaultRuntime().tokenUseDebouncer()
}

func resetTokenUseDebouncerForTest() {
	DefaultRuntime().resetTokenUseDebouncer()
}

func StopTokenUseDebouncer(ctx context.Context) error {
	debouncer := getTokenUseDebouncer()
	if debouncer == nil {
		return nil
	}
	return debouncer.Flush(ctx)
}

func (d *tokenUseDebouncer) Record(id uint, ip string, ts int64) {
	if id == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending[id] = tokenUseUpdate{ip: ip, ts: ts}
	d.scheduleLocked()
}

func (d *tokenUseDebouncer) scheduleLocked() {
	if d.timer != nil {
		return
	}
	epoch := d.epoch
	d.timer = time.AfterFunc(d.interval, func() {
		d.flushTimer(epoch)
	})
}

func (d *tokenUseDebouncer) flushTimer(epoch uint64) {
	updates := d.takePending(epoch)
	if len(updates) == 0 {
		return
	}
	if err := d.write(updates); err != nil {
		logger.Warning("token use flush failed:", err)
	}
	d.mu.Lock()
	if len(d.pending) > 0 {
		d.scheduleLocked()
	}
	d.mu.Unlock()
}

func (d *tokenUseDebouncer) Flush(ctx context.Context) error {
	d.mu.Lock()
	d.epoch++
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	updates := d.pending
	d.pending = make(map[uint]tokenUseUpdate)
	d.mu.Unlock()
	if len(updates) == 0 {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- d.write(updates)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *tokenUseDebouncer) takePending(epoch uint64) map[uint]tokenUseUpdate {
	d.mu.Lock()
	defer d.mu.Unlock()
	if epoch != d.epoch {
		return nil
	}
	updates := d.pending
	d.pending = make(map[uint]tokenUseUpdate)
	d.timer = nil
	return updates
}

func (d *tokenUseDebouncer) write(updates map[uint]tokenUseUpdate) error {
	if d.flush == nil || len(updates) == 0 {
		return nil
	}
	return d.flush(updates)
}

func flushTokenUseUpdates(updates map[uint]tokenUseUpdate) error {
	db := database.GetDB()
	if db == nil || len(updates) == 0 {
		return nil
	}
	ids := make([]int, 0, len(updates))
	for id := range updates {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for start := 0; start < len(ids); start += tokenUseBatchSize {
		end := start + tokenUseBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		if err := flushTokenUseBatch(ids[start:end], updates); err != nil {
			return err
		}
	}
	return nil
}

func flushTokenUseBatch(ids []int, updates map[uint]tokenUseUpdate) error {
	if len(ids) == 0 {
		return nil
	}
	var query strings.Builder
	args := make([]any, 0, len(ids)*5)
	query.WriteString("UPDATE tokens SET last_used_at = CASE id")
	for _, id := range ids {
		update := updates[uint(id)]
		query.WriteString(" WHEN ? THEN ?")
		args = append(args, id, update.ts)
	}
	query.WriteString(" END, last_used_ip = CASE id")
	for _, id := range ids {
		update := updates[uint(id)]
		query.WriteString(" WHEN ? THEN ?")
		args = append(args, id, update.ip)
	}
	query.WriteString(" END WHERE id IN (")
	for i, id := range ids {
		if i > 0 {
			query.WriteByte(',')
		}
		query.WriteByte('?')
		args = append(args, id)
	}
	query.WriteByte(')')
	if err := database.GetDB().Exec(query.String(), args...).Error; err != nil {
		return fmt.Errorf("flush token use batch: %w", err)
	}
	return nil
}
