package importxui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"gorm.io/gorm"
)

const (
	KindTLS      = "tls"
	KindInbound  = "inbound"
	KindEndpoint = "endpoint"
	KindClient   = "client"
	KindSetting  = "setting"
	KindAdmin    = "admin"
	KindHistory  = "historical"
	KindRouting  = "routing"

	ActionCreate  = "create"
	ActionMerge   = "merge"
	ActionReplace = "replace"
	ActionSkip    = "skip"
)

var (
	ErrPlanStale = errors.New("plan_stale")
	ErrBusy      = errors.New("xui_import_busy")
	applyMu      sync.Mutex
)

type MigrationPlan struct {
	Items    []PlanItem   `json:"items"`
	Defaults PlanDefaults `json:"defaults"`
	Source   PlanSource   `json:"source"`
}

type PlanDefaults struct {
	Strategy        string `json:"strategy"`
	IncludeSettings bool   `json:"includeSettings"`
	AdminMode       string `json:"adminMode"`
	OnlyNew         bool   `json:"onlyNew"`
	IncludeHistory  bool   `json:"includeHistory"`
	IncludeRouting  bool   `json:"includeRouting"`
}

type PlanSource struct {
	Path string `json:"path,omitempty"`
	Hash string `json:"hash"`
}

type PlanItem struct {
	Kind        string          `json:"kind"`
	SrcID       any             `json:"srcId"`
	SrcTag      string          `json:"srcTag,omitempty"`
	DstTag      string          `json:"dstTag"`
	Action      string          `json:"action"`
	Conflict    bool            `json:"conflict"`
	PreviewJSON json.RawMessage `json:"previewJson"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type Progress struct {
	Step        string `json:"step"`
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentTag  string `json:"currentTag,omitempty"`
	CurrentName string `json:"currentName,omitempty"`
	Percent     int    `json:"percent"`
}

func Plan(srcPath string, opts PlanOptions) (*MigrationPlan, error) {
	opts, err := opts.normalized()
	if err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	if err := checkContext(opts.Context); err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	src, err := openSource(srcPath)
	if err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	defer src.close()
	hash, err := hashSource(srcPath)
	if err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("xui-import: destination database is not initialized")
	}
	tx := db.Session(&gorm.Session{})
	state := &importState{
		report:          &Report{},
		realityByKey:    map[string]*realitySpec{},
		realityBySource: map[int64]*realitySpec{},
		tlsIDByKey:      map[string]uint{},
		inboundIDBySrc:  map[int64]uint{},
		server:          destinationServer(tx),
	}
	plan := &MigrationPlan{
		Defaults: PlanDefaults{
			Strategy:        string(opts.Strategy),
			IncludeSettings: opts.IncludeSettings,
			AdminMode:       string(opts.AdminMode),
			OnlyNew:         opts.OnlyNew,
			IncludeHistory:  opts.IncludeHistory,
			IncludeRouting:  opts.IncludeRouting,
		},
		Source: PlanSource{
			Path: srcPath,
			Hash: hash,
		},
	}
	if err := state.planTLS(opts.Context, tx, src, plan, opts.Strategy); err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	if err := state.planInboundsEndpoints(opts.Context, tx, src, plan, opts.Strategy); err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	if err := state.planClients(opts.Context, tx, src, plan, opts.Strategy); err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	if opts.IncludeSettings {
		if err := planSettings(opts.Context, tx, src, plan, opts.Strategy); err != nil {
			return nil, fmt.Errorf("xui-import: %w", err)
		}
	}
	if opts.AdminMode != AdminModeSkip {
		if err := planAdmins(opts.Context, tx, src, plan, opts.Strategy, opts.AdminMode); err != nil {
			return nil, fmt.Errorf("xui-import: %w", err)
		}
	}
	if opts.IncludeHistory {
		if err := planHistorical(opts.Context, src, plan); err != nil {
			return nil, fmt.Errorf("xui-import: %w", err)
		}
	}
	if opts.IncludeRouting {
		if err := planRouting(opts.Context, src, plan); err != nil {
			return nil, fmt.Errorf("xui-import: %w", err)
		}
	}
	if opts.OnlyNew {
		markOnlyNew(plan)
	}
	return plan, nil
}

func (s *importState) planTLS(ctx context.Context, tx *gorm.DB, src *sourceDB, plan *MigrationPlan, strategy Strategy) error {
	return src.eachInbound(func(row xuiInboundRow) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		spec, warnings, err := extractReality(row)
		if err != nil || spec == nil {
			return err
		}
		if existing, ok := s.realityByKey[spec.Key]; ok {
			s.realityBySource[row.ID] = existing
			return nil
		}
		s.realityByKey[spec.Key] = spec
		s.realityBySource[row.ID] = spec
		record, err := buildTLSRecord(*spec)
		if err != nil {
			return err
		}
		preview, err := marshalJSON(record)
		if err != nil {
			return err
		}
		_, conflict, err := findExistingRealityTLS(tx, *spec)
		if err != nil {
			return err
		}
		plan.Items = append(plan.Items, PlanItem{
			Kind:        KindTLS,
			SrcID:       spec.Key,
			SrcTag:      row.Tag,
			DstTag:      record.Name,
			Action:      defaultAction(conflict, strategy),
			Conflict:    conflict,
			PreviewJSON: preview,
			Warnings:    warnings,
		})
		return nil
	})
}

func (s *importState) planInboundsEndpoints(ctx context.Context, tx *gorm.DB, src *sourceDB, plan *MigrationPlan, strategy Strategy) error {
	return src.eachInbound(func(row xuiInboundRow) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if row.Protocol == "wireguard" {
			endpoint, warnings, err := mapWireguardEndpoint(row)
			if err != nil || endpoint == nil {
				if endpoint == nil {
					plan.Items = append(plan.Items, warningOnlyItem(KindEndpoint, row.ID, row.Tag, row.Tag, warnings))
				}
				return err
			}
			preview, err := marshalJSON(endpoint)
			if err != nil {
				return err
			}
			conflict, err := recordExists(tx, &model.Endpoint{}, "tag = ?", endpoint.Tag)
			if err != nil {
				return err
			}
			plan.Items = append(plan.Items, PlanItem{
				Kind:        KindEndpoint,
				SrcID:       row.ID,
				SrcTag:      row.Tag,
				DstTag:      endpoint.Tag,
				Action:      defaultAction(conflict, strategy),
				Conflict:    conflict,
				PreviewJSON: preview,
				Warnings:    warnings,
			})
			return nil
		}
		var reality *realitySpec
		if spec, ok := s.realityBySource[row.ID]; ok {
			reality = spec
		}
		mapped, err := mapInbound(row, 0, reality, s.server)
		if err != nil {
			return err
		}
		if mapped.Inbound.Type == "" {
			plan.Items = append(plan.Items, warningOnlyItem(KindInbound, row.ID, row.Tag, row.Tag, mapped.Warnings))
			return nil
		}
		preview, err := mapped.Inbound.MarshalFull()
		if err != nil {
			return err
		}
		previewJSON, err := marshalJSON(preview)
		if err != nil {
			return err
		}
		conflict, err := recordExists(tx, &model.Inbound{}, "tag = ?", mapped.Inbound.Tag)
		if err != nil {
			return err
		}
		s.inboundIDBySrc[row.ID] = uint(row.ID)
		for i := range mapped.ClientRefs {
			mapped.ClientRefs[i].DstInboundID = uint(row.ID)
		}
		s.clientRefs = append(s.clientRefs, mapped.ClientRefs...)
		plan.Items = append(plan.Items, PlanItem{
			Kind:        KindInbound,
			SrcID:       row.ID,
			SrcTag:      row.Tag,
			DstTag:      mapped.Inbound.Tag,
			Action:      defaultAction(conflict, strategy),
			Conflict:    conflict,
			PreviewJSON: previewJSON,
			Warnings:    mapped.Warnings,
		})
		return nil
	})
}

func (s *importState) planClients(ctx context.Context, tx *gorm.DB, src *sourceDB, plan *MigrationPlan, strategy Strategy) error {
	aggs, err := collectClientAggregates(src, s.clientRefs, s.inboundIDBySrc)
	if err != nil {
		return err
	}
	emails := make([]string, 0, len(aggs))
	for email := range aggs {
		emails = append(emails, email)
	}
	sortStrings(emails)
	for _, email := range emails {
		if err := checkContext(ctx); err != nil {
			return err
		}
		client, err := aggs[email].toModel()
		if err != nil {
			return err
		}
		preview, err := marshalJSON(client)
		if err != nil {
			return err
		}
		conflict, err := recordExists(tx, &model.Client{}, "name = ?", email)
		if err != nil {
			return err
		}
		plan.Items = append(plan.Items, PlanItem{
			Kind:        KindClient,
			SrcID:       email,
			SrcTag:      email,
			DstTag:      email,
			Action:      defaultAction(conflict, strategy),
			Conflict:    conflict,
			PreviewJSON: preview,
		})
	}
	return nil
}

func defaultAction(conflict bool, strategy Strategy) string {
	if !conflict {
		return ActionCreate
	}
	switch strategy {
	case StrategyReplace:
		return ActionReplace
	case StrategySkip:
		return ActionSkip
	default:
		return ActionMerge
	}
}

func warningOnlyItem(kind string, srcID any, srcTag string, dstTag string, warnings []string) PlanItem {
	return PlanItem{
		Kind:        kind,
		SrcID:       srcID,
		SrcTag:      srcTag,
		DstTag:      dstTag,
		Action:      ActionSkip,
		PreviewJSON: json.RawMessage(`null`),
		Warnings:    warnings,
	}
}

func recordExists(tx *gorm.DB, modelValue any, query string, args ...any) (bool, error) {
	var count int64
	if err := tx.Model(modelValue).Where(query, args...).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func Apply(srcPath string, plan MigrationPlan, opts ApplyOptions) (*Report, error) {
	opts = opts.normalized()
	report := &Report{}
	if !applyMu.TryLock() {
		return report, fmt.Errorf("xui-import: %w", ErrBusy)
	}
	defer applyMu.Unlock()
	if err := checkContext(opts.Context); err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	hash, err := hashSource(srcPath)
	if err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	if plan.Source.Hash != "" && plan.Source.Hash != hash {
		return report, fmt.Errorf("xui-import: %w", ErrPlanStale)
	}
	src, err := openSource(srcPath)
	if err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	defer src.close()
	db := database.GetDB()
	if db == nil {
		return report, fmt.Errorf("xui-import: destination database is not initialized")
	}
	var backupPath string
	if !opts.DryRun {
		now := time.Now().Unix()
		if opts.Now != nil {
			now = opts.Now()
		}
		backupPath, err = WritePreImportBackup(now)
		if err != nil {
			return report, err
		}
		report.BackupPath = backupPath
	}
	tx := db.Begin()
	if tx.Error != nil {
		return report, fmt.Errorf("xui-import: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback().Error
		}
	}()
	state := &applyState{
		report:          report,
		plan:            normalizePlan(plan),
		realityByKey:    map[string]*realitySpec{},
		realityBySource: map[int64]*realitySpec{},
		tlsIDByKey:      map[string]uint{},
		inboundIDBySrc:  map[int64]uint{},
		server:          destinationServer(tx),
		onProgress:      opts.OnProgress,
		total:           countRunnableItems(plan),
	}
	if err := state.run(opts.Context, tx, src, opts); err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	if opts.DryRun {
		return report, nil
	}
	if err := tx.Commit().Error; err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	committed = true
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return report, fmt.Errorf("xui-import: %w", err)
	}
	return report, nil
}

type applyState struct {
	report          *Report
	plan            map[string]PlanItem
	realityByKey    map[string]*realitySpec
	realityBySource map[int64]*realitySpec
	tlsIDByKey      map[string]uint
	inboundIDBySrc  map[int64]uint
	clientRefs      []ClientRef
	server          string
	onProgress      func(Progress)
	current         int
	total           int
}

func (s *applyState) run(ctx context.Context, tx *gorm.DB, src *sourceDB, opts ApplyOptions) error {
	total, err := src.inboundCount()
	if err != nil {
		return err
	}
	s.report.Summary.Inbounds.Total = total
	if err := s.applyTLS(ctx, tx, src); err != nil {
		return err
	}
	if err := s.applyInboundsEndpoints(ctx, tx, src); err != nil {
		return err
	}
	if err := s.applyClients(ctx, tx, src); err != nil {
		return err
	}
	if err := s.applySettings(ctx, tx, src); err != nil {
		return err
	}
	if err := s.applyAdmins(ctx, tx, src, opts); err != nil {
		return err
	}
	if err := s.applyHistorical(ctx, tx, src, opts); err != nil {
		return err
	}
	if err := s.applyRouting(ctx, tx, src, opts); err != nil {
		return err
	}
	if !opts.DryRun && !opts.SkipAudit {
		if err := recordAuditWithBackup(tx, s.report, opts); err != nil {
			return err
		}
		s.progress("audit", "xui_import")
	}
	return nil
}

func (s *applyState) applyTLS(ctx context.Context, tx *gorm.DB, src *sourceDB) error {
	return src.eachInbound(func(row xuiInboundRow) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		spec, warnings, err := extractReality(row)
		if err != nil || spec == nil {
			return err
		}
		if existing, ok := s.realityByKey[spec.Key]; ok {
			s.realityBySource[row.ID] = existing
			s.report.Summary.TLS.Reused++
			return nil
		}
		s.realityByKey[spec.Key] = spec
		s.realityBySource[row.ID] = spec
		item := s.item(KindTLS, spec.Key)
		s.report.warnAll(warnings)
		if item.Action == ActionSkip {
			return nil
		}
		record, err := buildTLSRecord(*spec)
		if err != nil {
			return err
		}
		if item.DstTag != "" {
			record.Name = item.DstTag
		}
		existing, found, err := findExistingRealityTLS(tx, *spec)
		if err != nil {
			return err
		}
		if found && item.Action != ActionReplace {
			s.tlsIDByKey[spec.Key] = existing.Id
			s.report.Summary.TLS.Reused++
			s.progress("tls", record.Name)
			return nil
		}
		if found && item.Action == ActionReplace {
			_ = tx.Delete(&existing).Error
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		s.tlsIDByKey[spec.Key] = record.Id
		s.report.Summary.TLS.Created++
		s.progress("tls", record.Name)
		return nil
	})
}

func (s *applyState) applyInboundsEndpoints(ctx context.Context, tx *gorm.DB, src *sourceDB) error {
	return src.eachInbound(func(row xuiInboundRow) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if row.Protocol == "wireguard" {
			endpoint, warnings, err := mapWireguardEndpoint(row)
			if err != nil {
				return err
			}
			s.report.warnAll(warnings)
			item := s.item(KindEndpoint, row.ID)
			if endpoint == nil || item.Action == ActionSkip {
				s.report.Summary.Inbounds.Skipped++
				return nil
			}
			if item.DstTag != "" {
				endpoint.Tag = item.DstTag
			}
			imported, err := applyEndpointAction(tx, endpoint, item.Action, s.report)
			if err != nil {
				return err
			}
			if imported {
				s.report.Summary.Endpoints.Imported++
			}
			s.progress("endpoints", endpoint.Tag)
			return nil
		}
		var tlsID uint
		var reality *realitySpec
		if spec, ok := s.realityBySource[row.ID]; ok {
			reality = spec
			tlsID = s.tlsIDByKey[spec.Key]
		}
		mapped, err := mapInbound(row, tlsID, reality, s.server)
		if err != nil {
			return err
		}
		s.report.warnAll(mapped.Warnings)
		item := s.item(KindInbound, row.ID)
		if mapped.Inbound.Type == "" || item.Action == ActionSkip {
			s.report.Summary.Inbounds.Skipped++
			return nil
		}
		if item.DstTag != "" {
			mapped.Inbound.Tag = item.DstTag
		}
		dstID, imported, skipped, err := applyInboundAction(tx, &mapped.Inbound, item.Action, s.report)
		if err != nil {
			return err
		}
		if skipped {
			s.report.Summary.Inbounds.Skipped++
			return nil
		}
		if imported {
			s.report.Summary.Inbounds.Imported++
		}
		s.inboundIDBySrc[row.ID] = dstID
		for i := range mapped.ClientRefs {
			mapped.ClientRefs[i].DstInboundID = dstID
		}
		s.clientRefs = append(s.clientRefs, mapped.ClientRefs...)
		s.report.ByInbound = append(s.report.ByInbound, InboundStat{
			SrcTag:  row.Tag,
			DstTag:  mapped.Inbound.Tag,
			Clients: len(mapped.ClientRefs),
		})
		s.progress("inbounds", mapped.Inbound.Tag)
		return nil
	})
}

func (s *applyState) applyClients(ctx context.Context, tx *gorm.DB, src *sourceDB) error {
	aggs, err := collectClientAggregates(src, s.clientRefs, s.inboundIDBySrc)
	if err != nil {
		return err
	}
	s.report.Summary.Clients.UniqueEmails = len(aggs)
	emails := make([]string, 0, len(aggs))
	for email := range aggs {
		emails = append(emails, email)
	}
	sortStrings(emails)
	for _, email := range emails {
		if err := checkContext(ctx); err != nil {
			return err
		}
		item := s.item(KindClient, email)
		if item.Action == ActionSkip {
			continue
		}
		if item.DstTag != "" && item.DstTag != email {
			renameAggregate(aggs[email], item.DstTag)
		}
		if err := applyClientAction(tx, aggs[email], item.Action, s.report); err != nil {
			return err
		}
		s.progress("clients", item.DstTag)
	}
	return nil
}

func (s *applyState) applySettings(ctx context.Context, tx *gorm.DB, src *sourceDB) error {
	if !s.hasKind(KindSetting) {
		return nil
	}
	settings, err := src.settings()
	if err != nil {
		return err
	}
	for _, setting := range settings {
		if err := checkContext(ctx); err != nil {
			return err
		}
		target, ok := mapSettingKey(setting.Key)
		if !ok {
			continue
		}
		item := s.item(KindSetting, setting.ID)
		if item.Action == ActionSkip {
			continue
		}
		if item.DstTag != "" {
			target = item.DstTag
		}
		if err := upsertSetting(tx, target, setting.Value); err != nil {
			return err
		}
		s.progress("settings", target)
	}
	return nil
}

func (s *applyState) applyAdmins(ctx context.Context, tx *gorm.DB, src *sourceDB, opts ApplyOptions) error {
	if !s.hasKind(KindAdmin) {
		return nil
	}
	users, err := src.users()
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := checkContext(ctx); err != nil {
			return err
		}
		item := s.item(KindAdmin, user.ID)
		if item.Action == ActionSkip {
			continue
		}
		username := firstNonEmpty(item.DstTag, user.Username)
		password := deterministicSeq(username+":admin:"+strconv.FormatInt(time.Now().UnixNano(), 10), 16)
		hash, err := common.HashPassword(password)
		if err != nil {
			return err
		}
		if err := upsertUser(tx, username, hash, item.Action); err != nil {
			return err
		}
		s.report.GeneratedAdmins = append(s.report.GeneratedAdmins, GeneratedAdmin{Username: username, Password: password})
		s.progress("admins", username)
	}
	return nil
}

func normalizePlan(plan MigrationPlan) map[string]PlanItem {
	items := map[string]PlanItem{}
	for _, item := range plan.Items {
		if item.Action == "" {
			item.Action = ActionCreate
		}
		items[planKey(item.Kind, item.SrcID)] = item
	}
	return items
}

func countRunnableItems(plan MigrationPlan) int {
	total := 0
	for _, item := range plan.Items {
		if item.Action != ActionSkip {
			total++
		}
	}
	if total == 0 {
		return 1
	}
	return total
}

func (s *applyState) item(kind string, srcID any) PlanItem {
	if item, ok := s.plan[planKey(kind, srcID)]; ok {
		return item
	}
	return PlanItem{Kind: kind, SrcID: srcID, Action: ActionCreate}
}

func (s *applyState) hasKind(kind string) bool {
	prefix := kind + ":"
	for key := range s.plan {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func planKey(kind string, srcID any) string {
	return kind + ":" + fmt.Sprint(srcID)
}

func srcIDInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}

func (s *applyState) progress(step string, name string) {
	if s.onProgress == nil {
		return
	}
	s.current++
	percent := 0
	if s.total > 0 {
		percent = s.current * 100 / s.total
		if percent > 100 {
			percent = 100
		}
	}
	event := Progress{
		Step:    step,
		Current: s.current,
		Total:   s.total,
		Percent: percent,
	}
	switch step {
	case "clients", "admins":
		event.CurrentName = name
	default:
		event.CurrentTag = name
	}
	s.onProgress(event)
}

func actionToStrategy(action string) Strategy {
	switch action {
	case ActionReplace:
		return StrategyReplace
	case ActionSkip:
		return StrategySkip
	default:
		return StrategyMerge
	}
}

func applyInboundAction(tx *gorm.DB, inbound *model.Inbound, action string, report *Report) (uint, bool, bool, error) {
	return applyInbound(tx, inbound, actionToStrategy(action), report)
}

func applyEndpointAction(tx *gorm.DB, endpoint *model.Endpoint, action string, report *Report) (bool, error) {
	return applyEndpoint(tx, endpoint, actionToStrategy(action), report)
}

func applyClientAction(tx *gorm.DB, agg *clientAggregate, action string, report *Report) error {
	return applyClient(tx, agg, actionToStrategy(action), report)
}

func renameAggregate(agg *clientAggregate, name string) {
	agg.Email = name
	for _, config := range agg.Config {
		if _, ok := config["name"]; ok {
			config["name"] = name
		}
		if _, ok := config["username"]; ok {
			config["username"] = name
		}
	}
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func planSettings(ctx context.Context, tx *gorm.DB, src *sourceDB, plan *MigrationPlan, strategy Strategy) error {
	settings, err := src.settings()
	if err != nil {
		return err
	}
	for _, setting := range settings {
		if err := checkContext(ctx); err != nil {
			return err
		}
		target, ok := mapSettingKey(setting.Key)
		if !ok {
			if setting.Key == "secretEnable" || strings.HasPrefix(strings.ToLower(setting.Key), "webcert") || strings.HasPrefix(strings.ToLower(setting.Key), "webkey") {
				plan.Items = append(plan.Items, warningOnlyItem(KindSetting, setting.ID, setting.Key, setting.Key, []string{fmt.Sprintf("setting %s requires manual review", setting.Key)}))
			}
			continue
		}
		preview := model.Setting{Key: target, Value: setting.Value}
		previewJSON, err := marshalJSON(preview)
		if err != nil {
			return err
		}
		conflict, err := recordExists(tx, &model.Setting{}, "key = ?", target)
		if err != nil {
			return err
		}
		plan.Items = append(plan.Items, PlanItem{
			Kind:        KindSetting,
			SrcID:       setting.ID,
			SrcTag:      setting.Key,
			DstTag:      target,
			Action:      defaultAction(conflict, strategy),
			Conflict:    conflict,
			PreviewJSON: previewJSON,
		})
	}
	return nil
}

func mapSettingKey(key string) (string, bool) {
	switch key {
	case "webPort", "webBasePath", "tgBotEnable", "tgBotToken", "tgBotChatId", "subEnable", "subPort", "subPath":
		return key, true
	case "tgRunTime":
		return "tgBotRunTime", true
	default:
		return "", false
	}
}

func upsertSetting(tx *gorm.DB, key string, value string) error {
	var setting model.Setting
	err := tx.Where("key = ?", key).First(&setting).Error
	if err != nil && !database.IsNotFound(err) {
		return err
	}
	if database.IsNotFound(err) {
		return tx.Create(&model.Setting{Key: key, Value: value}).Error
	}
	return tx.Model(&setting).Update("value", value).Error
}

func planAdmins(ctx context.Context, tx *gorm.DB, src *sourceDB, plan *MigrationPlan, strategy Strategy, mode AdminMode) error {
	users, err := src.users()
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := checkContext(ctx); err != nil {
			return err
		}
		preview := map[string]any{
			"username": user.Username,
			"mode":     mode,
		}
		previewJSON, err := marshalJSON(preview)
		if err != nil {
			return err
		}
		conflict, err := recordExists(tx, &model.User{}, "username = ?", user.Username)
		if err != nil {
			return err
		}
		plan.Items = append(plan.Items, PlanItem{
			Kind:        KindAdmin,
			SrcID:       user.ID,
			SrcTag:      user.Username,
			DstTag:      user.Username,
			Action:      defaultAction(conflict, strategy),
			Conflict:    conflict,
			PreviewJSON: previewJSON,
		})
	}
	return nil
}

func upsertUser(tx *gorm.DB, username string, passwordHash string, action string) error {
	var user model.User
	err := tx.Where("username = ?", username).First(&user).Error
	if err != nil && !database.IsNotFound(err) {
		return err
	}
	if database.IsNotFound(err) {
		return tx.Create(&model.User{Username: username, Password: passwordHash}).Error
	}
	if action == ActionSkip || action == "" {
		return nil
	}
	return tx.Model(&user).Update("password", passwordHash).Error
}

func recordAuditWithBackup(tx *gorm.DB, report *Report, opts ApplyOptions) error {
	now := time.Now().Unix()
	if opts.Now != nil {
		now = opts.Now()
	}
	details := summaryDetails(report.Summary)
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return tx.Create(&model.AuditEvent{
		DateTime: now,
		Actor:    "system",
		Event:    "xui_import",
		Resource: "database",
		Severity: "info",
		Details:  raw,
	}).Error
}

func sortPlanItems(items []PlanItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return fmt.Sprint(items[i].SrcID) < fmt.Sprint(items[j].SrcID)
	})
}
