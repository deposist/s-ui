package service

import (
	"encoding/json"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/util/redact"
)

const (
	AuditSeverityInfo = "info"
	AuditSeverityWarn = "warn"
)

type AuditService struct {
}

type AuditEvent struct {
	Actor     string
	Event     string
	Resource  string
	Severity  string
	IP        string
	UserAgent string
	Details   map[string]any
}

func (s *AuditService) Record(event AuditEvent) error {
	if event.Severity == "" {
		event.Severity = AuditSeverityInfo
	}
	details, err := json.Marshal(redact.Value(event.Details))
	if err != nil {
		return err
	}
	return database.GetDB().Create(&model.AuditEvent{
		DateTime:  time.Now().Unix(),
		Actor:     event.Actor,
		Event:     event.Event,
		Resource:  event.Resource,
		Severity:  event.Severity,
		IP:        event.IP,
		UserAgent: event.UserAgent,
		Details:   details,
	}).Error
}

func (s *AuditService) List(limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	events := make([]model.AuditEvent, 0)
	err := database.GetDB().Model(model.AuditEvent{}).
		Order("date_time desc, id desc").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (s *AuditService) Prune(retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	before := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	return database.GetDB().Where("date_time < ?", before).Delete(&model.AuditEvent{}).Error
}
