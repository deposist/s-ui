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
)

var pending = struct {
	sync.Mutex
	byClient map[string]map[string]int64
}{
	byClient: map[string]map[string]int64{},
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
}

func Allow(clientName string, ip string) bool {
	if clientName == "" || ip == "" {
		return true
	}
	db := database.GetDB()
	if db == nil {
		return true
	}
	var client model.Client
	if err := db.Model(model.Client{}).Select("limit_ip, ip_limit_mode").Where("name = ?", clientName).First(&client).Error; err != nil {
		return true
	}
	if client.IPLimitMode != ModeEnforce || client.LimitIP <= 0 {
		return true
	}
	seen := map[string]struct{}{ip: {}}
	var ips []string
	_ = db.Model(model.ClientIP{}).Where("client_name = ?", clientName).Pluck("ip", &ips).Error
	for _, seenIP := range ips {
		seen[seenIP] = struct{}{}
	}
	pending.Lock()
	for seenIP := range pending.byClient[clientName] {
		seen[seenIP] = struct{}{}
	}
	pending.Unlock()
	return len(seen) <= client.LimitIP
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
	return db.Model(model.Client{}).Where("name = ?", clientName).Updates(map[string]interface{}{
		"last_ip_count": 0,
	}).Error
}
