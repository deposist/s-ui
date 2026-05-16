package service

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

type countingRoundTripper struct {
	count atomic.Int32
}

func (r *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	r.count.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     http.Header{},
	}, nil
}

func (r *countingRoundTripper) Count() int {
	return int(r.count.Load())
}

func TestTelegramDisabledMakesNoOutboundCall(t *testing.T) {
	initSettingTestDB(t)
	rt := &countingRoundTripper{}
	t.Cleanup(setTelegramHTTPClient(&http.Client{Transport: rt, Timeout: time.Second}))

	result := (&TelegramService{}).TestTelegram()
	if result.Success || result.ErrorClass != "disabled" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if rt.Count() != 0 {
		t.Fatalf("disabled telegram made %d outbound calls", rt.Count())
	}
}

func TestNotifyTelegramEventReturnsBeforeSendCompletes(t *testing.T) {
	settingService := initSettingTestDB(t)
	enableTelegramForTest(t, settingService)

	sendStarted := make(chan struct{})
	releaseSend := make(chan struct{})
	sendDone := make(chan struct{})
	var startedOnce sync.Once
	var doneOnce sync.Once
	notifier := newTelegramNotifier(telegramQueueCapacity, func(string) TelegramResult {
		startedOnce.Do(func() { close(sendStarted) })
		<-releaseSend
		doneOnce.Do(func() { close(sendDone) })
		return TelegramResult{Success: true}
	}, func(string, map[string]any) {})
	notifier.backoff = nil
	replaceDefaultTelegramNotifierForTest(t, notifier)

	start := time.Now()
	(&TelegramService{}).NotifyTelegramEvent("login_failed", map[string]string{
		"ip": "203.0.113.10",
	})
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("NotifyTelegramEvent blocked for %s", elapsed)
	}

	select {
	case <-sendStarted:
	case <-time.After(time.Second):
		t.Fatal("queued notification was not delivered to worker")
	}
	select {
	case <-sendDone:
		t.Fatal("send completed before release; test did not exercise async path")
	default:
	}
	close(releaseSend)
	select {
	case <-sendDone:
	case <-time.After(time.Second):
		t.Fatal("send did not complete after release")
	}
}

func TestNotifyTelegramEventRedactsSensitiveFields(t *testing.T) {
	settingService := initSettingTestDB(t)
	enableTelegramForTest(t, settingService)

	sent := make(chan string, 1)
	notifier := newTelegramNotifier(telegramQueueCapacity, func(text string) TelegramResult {
		sent <- text
		return TelegramResult{Success: true}
	}, func(string, map[string]any) {})
	notifier.backoff = nil
	replaceDefaultTelegramNotifierForTest(t, notifier)

	(&TelegramService{}).NotifyTelegramEvent("manual_backup", map[string]string{
		"caption":       "Authorization: Bearer secret.jwt.value",
		"telegramToken": "1234567890:" + strings.Repeat("A", 35),
	})

	got := receiveString(t, sent, "redacted notification")
	if strings.Contains(got, "secret.jwt.value") || strings.Contains(got, "1234567890:") {
		t.Fatalf("telegram notification leaked sensitive value: %q", got)
	}
	if !strings.Contains(got, "Authorization: Bearer [REDACTED]") ||
		!strings.Contains(got, "telegramToken: [REDACTED]") {
		t.Fatalf("telegram notification was not redacted: %q", got)
	}
}

func TestTelegramNotifierRetriesAndAuditsFailure(t *testing.T) {
	type auditRecord struct {
		event   string
		details map[string]any
	}
	auditCh := make(chan auditRecord, 1)
	var attempts atomic.Int32
	notifier := newTelegramNotifier(4, func(string) TelegramResult {
		attempts.Add(1)
		return TelegramResult{ErrorClass: "network"}
	}, func(event string, details map[string]any) {
		auditCh <- auditRecord{event: event, details: details}
	})
	notifier.backoff = []time.Duration{time.Millisecond, time.Millisecond}

	notifier.Enqueue(telegramNotification{event: "login_failed", text: "message body"})

	var record auditRecord
	select {
	case record = <-auditCh:
	case <-time.After(time.Second):
		t.Fatal("notifier_failed audit was not recorded")
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
	if record.event != "notifier_failed" {
		t.Fatalf("unexpected audit event: %s", record.event)
	}
	if record.details["channel"] != "telegram" ||
		record.details["event"] != "login_failed" ||
		record.details["errorClass"] != "network" ||
		record.details["attempts"] != 3 {
		t.Fatalf("unexpected audit details: %#v", record.details)
	}
	if _, ok := record.details["text"]; ok {
		t.Fatalf("message text leaked to audit details: %#v", record.details)
	}
}

func TestTelegramNotifierDropsOldestAndAuditsOverflow(t *testing.T) {
	type auditRecord struct {
		event   string
		details map[string]any
	}
	auditCh := make(chan auditRecord, 1)
	sent := make(chan string, 4)
	releaseFirst := make(chan struct{})
	var blockFirst sync.Once
	notifier := newTelegramNotifier(2, func(text string) TelegramResult {
		sent <- text
		blockFirst.Do(func() { <-releaseFirst })
		return TelegramResult{Success: true}
	}, func(event string, details map[string]any) {
		auditCh <- auditRecord{event: event, details: details}
	})
	notifier.backoff = nil

	notifier.Enqueue(telegramNotification{event: "e1", text: "e1"})
	if got := receiveString(t, sent, "first send"); got != "e1" {
		t.Fatalf("unexpected first send: %s", got)
	}
	notifier.Enqueue(telegramNotification{event: "e2", text: "e2"})
	notifier.Enqueue(telegramNotification{event: "e3", text: "e3"})
	notifier.Enqueue(telegramNotification{event: "e4", text: "e4"})

	var record auditRecord
	select {
	case record = <-auditCh:
	case <-time.After(time.Second):
		t.Fatal("notifier_overflow audit was not recorded")
	}
	if record.event != "notifier_overflow" {
		t.Fatalf("unexpected audit event: %s", record.event)
	}
	if record.details["channel"] != "telegram" ||
		record.details["droppedEvent"] != "e2" ||
		record.details["queuedEvent"] != "e4" {
		t.Fatalf("unexpected overflow details: %#v", record.details)
	}

	close(releaseFirst)
	if got := receiveString(t, sent, "second send"); got != "e3" {
		t.Fatalf("drop-oldest should keep e3, got %s", got)
	}
	if got := receiveString(t, sent, "third send"); got != "e4" {
		t.Fatalf("drop-oldest should keep e4, got %s", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("dropped event was delivered: %s", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func enableTelegramForTest(t *testing.T, settingService *SettingService) {
	t.Helper()
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{
		"telegramEnabled":  "true",
		"telegramBotToken": "123456:test-token",
		"telegramChatID":   "42",
	}
	for key, value := range settings {
		if err := database.GetDB().Model(model.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func replaceDefaultTelegramNotifierForTest(t *testing.T, notifier *telegramNotifier) {
	t.Helper()
	oldNotifier := defaultTelegramNotifier
	defaultTelegramNotifier = notifier
	t.Cleanup(func() {
		defaultTelegramNotifier = oldNotifier
	})
}

func receiveString(t *testing.T, ch <-chan string, label string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %s", label)
		return ""
	}
}
