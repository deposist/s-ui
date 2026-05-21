package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/importxui"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/realtime"
	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-gonic/gin"
)

const (
	maxXUIImportBytes = 200 << 20
	maxXUIFieldBytes  = 8 << 20
	xuiRequestWindow  = time.Minute
	xuiRequestMax     = 5
	xuiRequestTimeout = 10 * time.Minute
)

type xuiUpload struct {
	Path   string
	Dir    string
	SHA256 string
	Fields map[string]string
}

type xuiAttempt struct {
	Count    int
	WindowAt time.Time
}

type xuiFieldTooLargeError struct {
	Field string
	Limit int64
}

func (e *xuiFieldTooLargeError) Error() string {
	return "payload_too_large: field " + e.Field + " exceeds " + strconv.FormatInt(e.Limit, 10) + " bytes"
}

var (
	xuiRateMu sync.Mutex
	xuiRates  = map[string]xuiAttempt{}
)

func init() {
	database.RegisterResetHook("api.xui_rates", resetXUIRateLimitCache)
}

func resetXUIRateLimitCache() {
	xuiRateMu.Lock()
	defer xuiRateMu.Unlock()
	xuiRates = map[string]xuiAttempt{}
}

func (a *ApiService) ImportXui(c *gin.Context) {
	ctx, cancel, ok := a.beginXUIRequest(c)
	if !ok {
		return
	}
	defer cancel()
	upload, err := saveXUIUpload(c)
	if err != nil {
		a.recordXuiImportFailure(c, err, "")
		xuiImportError(c, err)
		return
	}
	defer os.RemoveAll(upload.Dir)

	dryRun := upload.Fields["dryRun"] == "1"
	strategy := importxui.Strategy(upload.Fields["strategy"])
	if strategy == "" {
		strategy = importxui.StrategyMerge
	}
	if err := strategy.Validate(); err != nil {
		a.recordXuiImportFailure(c, err, upload.SHA256)
		xuiImportError(c, err)
		return
	}
	var backupPath string
	if !dryRun {
		var err error
		backupPath, err = importxui.WritePreImportBackup(time.Now().Unix())
		if err != nil {
			a.recordXuiImportFailure(c, err, upload.SHA256)
			xuiImportError(c, err)
			return
		}
	}
	report, err := importxui.Import(upload.Path, importxui.Options{
		Context:   ctx,
		DryRun:    dryRun,
		Strategy:  strategy,
		SkipAudit: true,
	})
	if err != nil {
		a.recordXuiImportFailure(c, err, upload.SHA256)
		xuiImportError(c, err)
		return
	}
	report.BackupPath = backupPath
	if !dryRun {
		a.recordAudit(c, requestActor(c), "xui_import", "database", service.AuditSeverityInfo, reportAuditDetails(report, upload.SHA256))
	}
	jsonObj(c, report, nil)
}

func (a *ApiService) ImportXuiPlan(c *gin.Context) {
	ctx, cancel, ok := a.beginXUIRequest(c)
	if !ok {
		return
	}
	defer cancel()
	upload, err := saveXUIUpload(c)
	if err != nil {
		a.recordXuiImportFailure(c, err, "")
		xuiImportError(c, err)
		return
	}
	defer os.RemoveAll(upload.Dir)

	strategy := importxui.Strategy(upload.Fields["strategy"])
	if strategy == "" {
		strategy = importxui.StrategyMerge
	}
	adminMode := importxui.AdminMode(upload.Fields["adminMode"])
	if adminMode == "" {
		adminMode = importxui.AdminModeSkip
	}
	plan, err := importxui.Plan(upload.Path, importxui.PlanOptions{
		Context:         ctx,
		Strategy:        strategy,
		IncludeSettings: upload.Fields["includeSettings"] == "1",
		IncludeHistory:  upload.Fields["includeHistory"] == "1",
		IncludeRouting:  upload.Fields["includeRouting"] == "1",
		AdminMode:       adminMode,
	})
	if err != nil {
		a.recordXuiImportFailure(c, err, upload.SHA256)
		xuiImportError(c, err)
		return
	}
	plan.Source.Path = ""
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(http.StatusOK)
	_ = json.NewEncoder(c.Writer).Encode(Msg{Success: true, Obj: plan})
}

func (a *ApiService) ImportXuiApply(c *gin.Context) {
	ctx, cancel, ok := a.beginXUIRequest(c)
	if !ok {
		return
	}
	defer cancel()
	upload, err := saveXUIUpload(c)
	if err != nil {
		a.recordXuiImportFailure(c, err, "")
		xuiImportError(c, err)
		return
	}
	defer os.RemoveAll(upload.Dir)

	var plan importxui.MigrationPlan
	decoder := json.NewDecoder(strings.NewReader(upload.Fields["plan"]))
	decoder.UseNumber()
	if err := decoder.Decode(&plan); err != nil {
		a.recordXuiImportFailure(c, err, upload.SHA256)
		xuiImportError(c, err)
		return
	}
	report, err := importxui.Apply(upload.Path, plan, importxui.ApplyOptions{
		Context:   ctx,
		SkipAudit: true,
		OnProgress: func(progress importxui.Progress) {
			realtime.Publish(realtime.TopicXUIImportProgress, progress)
		},
	})
	if err != nil {
		a.recordXuiImportFailure(c, err, upload.SHA256)
		xuiImportError(c, err)
		return
	}
	a.recordAudit(c, requestActor(c), "xui_import", "database", service.AuditSeverityInfo, reportAuditDetails(report, upload.SHA256))
	jsonObj(c, report, nil)
}

func (a *ApiService) ImportXuiRollback(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return
	}
	if !a.enforceXUIRateLimit(c) {
		return
	}
	backupPath := c.PostForm("backup")
	if backupPath == "" {
		backupPath = c.Query("backup")
	}
	if err := validateRollbackPath(backupPath); err != nil {
		a.recordAudit(c, requestActor(c), "xui_import_failed", "database", service.AuditSeverityWarn, map[string]any{"reason": "invalid_backup"})
		xuiImportError(c, err)
		return
	}
	file, err := os.Open(backupPath)
	if err != nil {
		a.recordXuiImportFailure(c, err, "")
		xuiImportError(c, err)
		return
	}
	defer file.Close()
	if err := database.ImportDB(multipart.File(file)); err != nil {
		a.recordXuiImportFailure(c, err, "")
		xuiImportError(c, err)
		return
	}
	a.recordAudit(c, requestActor(c), "xui_import_rollback", "database", service.AuditSeverityWarn, map[string]any{
		"backup": filepath.Base(backupPath),
	})
	jsonMsg(c, "import-xui", nil)
}

func (a *ApiService) ImportXuiReports(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return
	}
	if !a.enforceXUIRateLimit(c) {
		return
	}
	var events []model.AuditEvent
	err := database.GetDB().
		Where("event IN ?", []string{"xui_import", "xui_import_failed", "xui_import_rollback"}).
		Order("date_time desc").
		Limit(50).
		Find(&events).Error
	jsonObj(c, events, err)
}

func (a *ApiService) beginXUIRequest(c *gin.Context) (context.Context, context.CancelFunc, bool) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return c.Request.Context(), func() {}, false
	}
	if !a.enforceXUIRateLimit(c) {
		return c.Request.Context(), func() {}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), xuiRequestTimeout)
	c.Request = c.Request.WithContext(ctx)
	return ctx, cancel, true
}

func (a *ApiService) enforceXUIRateLimit(c *gin.Context) bool {
	key := requestActor(c)
	if key == "" {
		key = getRemoteIp(c)
	}
	xuiRateMu.Lock()
	defer xuiRateMu.Unlock()
	now := time.Now()
	attempt := xuiRates[key]
	if attempt.WindowAt.IsZero() || now.Sub(attempt.WindowAt) >= xuiRequestWindow {
		attempt = xuiAttempt{WindowAt: now}
	}
	if attempt.Count >= xuiRequestMax {
		xuiRates[key] = attempt
		a.recordAudit(c, requestActor(c), "xui_import_failed", "database", service.AuditSeverityWarn, map[string]any{"reason": "rate_limited"})
		c.JSON(http.StatusTooManyRequests, Msg{Success: false, Msg: "too many xui import requests"})
		return false
	}
	attempt.Count++
	xuiRates[key] = attempt
	return true
}

func saveXUIUpload(c *gin.Context) (*xuiUpload, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxXUIImportBytes)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(os.TempDir(), "xui-import-*")
	if err != nil {
		return nil, err
	}
	upload := &xuiUpload{Dir: dir, Fields: map[string]string{}}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		name := part.FormName()
		if name == "db" {
			path := filepath.Join(dir, "source.db")
			out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				_ = os.RemoveAll(dir)
				return nil, err
			}
			hash := sha256.New()
			_, copyErr := io.Copy(out, io.TeeReader(part, hash))
			closeErr := out.Close()
			if copyErr != nil {
				_ = os.RemoveAll(dir)
				return nil, copyErr
			}
			if closeErr != nil {
				_ = os.RemoveAll(dir)
				return nil, closeErr
			}
			if err := validateSQLiteFile(path); err != nil {
				_ = os.RemoveAll(dir)
				return nil, err
			}
			upload.Path = path
			upload.SHA256 = hex.EncodeToString(hash.Sum(nil))
			continue
		}
		value, err := readXUIField(part, name, maxXUIFieldBytes)
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		upload.Fields[name] = value
	}
	if upload.Path == "" {
		_ = os.RemoveAll(dir)
		return nil, errors.New("missing db file")
	}
	return upload, nil
}

func readXUIField(part *multipart.Part, name string, limit int64) (string, error) {
	value, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(value)) > limit {
		return "", &xuiFieldTooLargeError{Field: name, Limit: limit}
	}
	return string(value), nil
}

func validateSQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	ok, err := database.IsSQLiteDB(file)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("not_sqlite")
	}
	return nil
}

func (a *ApiService) recordXuiImportFailure(c *gin.Context, err error, sha string) {
	details := map[string]any{
		"reason": xuiImportErrorClass(err),
	}
	if sha != "" {
		details["sha256"] = sha
	}
	if errors.Is(err, importxui.ErrBusy) {
		a.recordAudit(c, requestActor(c), "xui_import_busy", "database", service.AuditSeverityWarn, details)
		return
	}
	a.recordAudit(c, requestActor(c), "xui_import_failed", "database", service.AuditSeverityWarn, details)
}

func xuiImportError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	var maxBytesErr *http.MaxBytesError
	var fieldTooLargeErr *xuiFieldTooLargeError
	switch {
	case errors.As(err, &maxBytesErr):
		status = http.StatusRequestEntityTooLarge
	case errors.As(err, &fieldTooLargeErr):
		status = http.StatusRequestEntityTooLarge
	case strings.Contains(err.Error(), "request body too large"):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, importxui.ErrBusy):
		status = http.StatusTooManyRequests
	case errors.Is(err, importxui.ErrPlanStale) || strings.Contains(err.Error(), "plan_stale"):
		status = http.StatusBadRequest
	}
	c.JSON(status, Msg{
		Success: false,
		Msg:     "import-xui: " + err.Error(),
	})
}

func xuiImportErrorClass(err error) string {
	var maxBytesErr *http.MaxBytesError
	var fieldTooLargeErr *xuiFieldTooLargeError
	switch {
	case errors.As(err, &maxBytesErr), errors.As(err, &fieldTooLargeErr), strings.Contains(err.Error(), "request body too large"):
		return "payload_too_large"
	case errors.Is(err, importxui.ErrBusy), strings.Contains(err.Error(), "xui_import_busy"):
		return "busy"
	case errors.Is(err, importxui.ErrPlanStale), strings.Contains(err.Error(), "plan_stale"):
		return "plan_stale"
	case strings.Contains(err.Error(), "not_sqlite"), strings.Contains(strings.ToLower(err.Error()), "sqlite"):
		return "not_sqlite"
	default:
		return "failed"
	}
}

func reportAuditDetails(report *importxui.Report, sha string) map[string]any {
	details := summaryDetailsForAPI(report.Summary)
	if sha != "" {
		details["sha256"] = sha
	}
	return details
}

func validateRollbackPath(path string) error {
	if path == "" {
		return errors.New("missing backup path")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("invalid backup path")
	}
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	realPath, err = filepath.Abs(realPath)
	if err != nil {
		return err
	}
	baseDir, err := filepath.Abs(filepath.Dir(config.GetDBPath()))
	if err != nil {
		return err
	}
	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return err
	}
	realBaseDir, err = filepath.Abs(realBaseDir)
	if err != nil {
		return err
	}
	if filepath.Dir(realPath) != realBaseDir || !strings.HasPrefix(filepath.Base(realPath), "s-ui-pre-xui-import-") || filepath.Ext(realPath) != ".db" {
		return errors.New("invalid backup path")
	}
	return nil
}

func summaryDetailsForAPI(summary importxui.Summary) map[string]any {
	return map[string]any{
		"inbounds": map[string]any{
			"total":     summary.Inbounds.Total,
			"imported":  summary.Inbounds.Imported,
			"skipped":   summary.Inbounds.Skipped,
			"conflicts": summary.Inbounds.Conflicts,
		},
		"endpoints": map[string]any{
			"imported": summary.Endpoints.Imported,
		},
		"tls": map[string]any{
			"created": summary.TLS.Created,
			"reused":  summary.TLS.Reused,
		},
		"clients": map[string]any{
			"unique_emails": summary.Clients.UniqueEmails,
			"merged":        summary.Clients.Merged,
			"created":       summary.Clients.Created,
		},
	}
}
