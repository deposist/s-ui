package ipmonitor

import (
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"gorm.io/gorm"
)

const (
	ModeMonitor = "monitor"
	ModeEnforce = "enforce"

	allowCacheTTL = 30 * time.Second
)

var pending = struct {
	sync.Mutex
	byClient map[string]map[string]int64
}{
	byClient: map[string]map[string]int64{},
}

type allowCacheEntry struct {
	limit     int
	mode      string
	ips       map[string]struct{}
	expiresAt time.Time
}

var allowCache = struct {
	sync.Mutex
	byClient map[string]allowCacheEntry
}{
	byClient: map[string]allowCacheEntry{},
}

func Record(clientName string, ip string) {
	if clientName == "" || ip == "" {
		return
	}
	now := time.Now().Unix()
	pending.Lock()
	if pending.byClient[clientName] == nil {
		pending.byClient[clientName] = map[string]int64{}
	}
	pending.byClient[clientName][ip] = now
	pending.Unlock()
	cacheAddIP(clientName, ip)
}

func Allow(clientName string, ip string) bool {
	if clientName == "" || ip == "" {
		return true
	}
	db := database.GetDB()
	if db == nil {
		return true
	}
	entry, ok := cachedClient(clientName, time.Now())
	if !ok {
		return true
	}
	if entry.mode != ModeEnforce || entry.limit <= 0 {
		return true
	}
	seen := map[string]struct{}{ip: {}}
	for seenIP := range entry.ips {
		seen[seenIP] = struct{}{}
	}
	pending.Lock()
	for seenIP := range pending.byClient[clientName] {
		seen[seenIP] = struct{}{}
	}
	pending.Unlock()
	return len(seen) <= entry.limit
}

func Flush() error {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	pending.Lock()
	snapshot := pending.byClient
	pending.byClient = map[string]map[string]int64{}
	pending.Unlock()
	if len(snapshot) == 0 {
		return nil
	}
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if err := flushSnapshot(tx, snapshot); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func FlushTo(tx *gorm.DB) error {
	pending.Lock()
	snapshot := pending.byClient
	pending.byClient = map[string]map[string]int64{}
	pending.Unlock()
	if len(snapshot) == 0 {
		return nil
	}
	return flushSnapshot(tx, snapshot)
}

func flushSnapshot(tx *gorm.DB, snapshot map[string]map[string]int64) error {
	for clientName, ips := range snapshot {
		lastSeen := int64(0)
		for ip, seenAt := range ips {
			if seenAt > lastSeen {
				lastSeen = seenAt
			}
			var row model.ClientIP
			err := tx.Model(model.ClientIP{}).Where("client_name = ? AND ip = ?", clientName, ip).First(&row).Error
			if database.IsNotFound(err) {
				err = tx.Create(&model.ClientIP{
					ClientName: clientName,
					IP:         ip,
					FirstSeen:  seenAt,
					LastSeen:   seenAt,
				}).Error
			} else if err == nil {
				err = tx.Model(model.ClientIP{}).Where("id = ?", row.Id).Update("last_seen", seenAt).Error
			}
			if err != nil {
				return err
			}
			cacheAddIP(clientName, ip)
		}
		var count int64
		if err := tx.Model(model.ClientIP{}).Where("client_name = ?", clientName).Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(model.Client{}).Where("name = ?", clientName).Updates(map[string]interface{}{
			"last_online":   lastSeen,
			"last_ip_count": count,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func History(clientName string, limit int) ([]model.ClientIP, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows := make([]model.ClientIP, 0)
	err := database.GetDB().Model(model.ClientIP{}).
		Where("client_name = ?", clientName).
		Order("last_seen desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func Clear(clientName string) error {
	db := database.GetDB()
	if err := db.Where("client_name = ?", clientName).Delete(&model.ClientIP{}).Error; err != nil {
		return err
	}
	invalidateCache(clientName)
	return db.Model(model.Client{}).Where("name = ?", clientName).Updates(map[string]interface{}{
		"last_ip_count": 0,
	}).Error
}

func cachedClient(clientName string, now time.Time) (allowCacheEntry, bool) {
	allowCache.Lock()
	defer allowCache.Unlock()
	if entry, ok := allowCache.byClient[clientName]; ok && now.Before(entry.expiresAt) {
		return cloneCacheEntry(entry), true
	}
	entry, ok := loadCacheEntry(clientName, now)
	if !ok {
		delete(allowCache.byClient, clientName)
		return allowCacheEntry{}, false
	}
	allowCache.byClient[clientName] = entry
	return cloneCacheEntry(entry), true
}

func loadCacheEntry(clientName string, now time.Time) (allowCacheEntry, bool) {
	db := database.GetDB()
	if db == nil {
		return allowCacheEntry{}, false
	}
	var client model.Client
	if err := db.Model(model.Client{}).Select("limit_ip, ip_limit_mode").Where("name = ?", clientName).First(&client).Error; err != nil {
		return allowCacheEntry{}, false
	}
	entry := allowCacheEntry{
		limit:     client.LimitIP,
		mode:      client.IPLimitMode,
		ips:       map[string]struct{}{},
		expiresAt: now.Add(allowCacheTTL),
	}
	var ips []string
	_ = db.Model(model.ClientIP{}).Where("client_name = ?", clientName).Pluck("ip", &ips).Error
	for _, ip := range ips {
		entry.ips[ip] = struct{}{}
	}
	return entry, true
}

func cloneCacheEntry(entry allowCacheEntry) allowCacheEntry {
	clone := allowCacheEntry{
		limit:     entry.limit,
		mode:      entry.mode,
		ips:       make(map[string]struct{}, len(entry.ips)),
		expiresAt: entry.expiresAt,
	}
	for ip := range entry.ips {
		clone.ips[ip] = struct{}{}
	}
	return clone
}

func cacheAddIP(clientName string, ip string) {
	allowCache.Lock()
	defer allowCache.Unlock()
	entry, ok := allowCache.byClient[clientName]
	if !ok || time.Now().After(entry.expiresAt) {
		return
	}
	if entry.ips == nil {
		entry.ips = map[string]struct{}{}
	}
	entry.ips[ip] = struct{}{}
	allowCache.byClient[clientName] = entry
}

func invalidateCache(clientName string) {
	allowCache.Lock()
	defer allowCache.Unlock()
	delete(allowCache.byClient, clientName)
}
