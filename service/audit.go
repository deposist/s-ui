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
	events, _, err := s.ListPage(0, limit)
	return events, err
}

func (s *AuditService) ListPage(cursor uint64, limit int) ([]model.AuditEvent, uint64, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 200 {
		limit = 200
	}
	events := make([]model.AuditEvent, 0, limit+1)
	query := database.GetDB().Model(model.AuditEvent{})
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	err := query.
		Order("id desc").
		Limit(limit + 1).
		Find(&events).Error
	if err != nil {
		return nil, 0, err
	}
	var nextCursor uint64
	if len(events) > limit {
		events = events[:limit]
		nextCursor = events[len(events)-1].Id
	}
	return events, nextCursor, nil
}

func (s *AuditService) Prune(retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}
	before := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	return database.GetDB().Where("date_time < ?", before).Delete(&model.AuditEvent{}).Error
}
