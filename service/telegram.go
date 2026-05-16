package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/common"
)

type TelegramService struct {
	SettingService
}

type TelegramResult struct {
	Success    bool   `json:"success"`
	ErrorClass string `json:"errorClass,omitempty"`
}

const telegramQueueCapacity = 256

var (
	telegramHTTPClientMu sync.RWMutex
	telegramHTTPClient   = &http.Client{Timeout: 10 * time.Second}

	defaultTelegramNotifier = newTelegramNotifier(
		telegramQueueCapacity,
		func(text string) TelegramResult {
			return (&TelegramService{}).send(text)
		},
		recordTelegramNotifierAudit,
	)
)

type telegramNotification struct {
	event string
	text  string
}

type telegramNotifier struct {
	capacity int
	send     func(string) TelegramResult
	audit    func(string, map[string]any)
	backoff  []time.Duration

	mu      sync.Mutex
	cond    *sync.Cond
	queue   []telegramNotification
	started bool
}

func newTelegramNotifier(capacity int, send func(string) TelegramResult, audit func(string, map[string]any)) *telegramNotifier {
	if capacity <= 0 {
		capacity = telegramQueueCapacity
	}
	notifier := &telegramNotifier{
		capacity: capacity,
		send:     send,
		audit:    audit,
		backoff: []time.Duration{
			500 * time.Millisecond,
			2 * time.Second,
		},
		queue: make([]telegramNotification, 0, capacity),
	}
	notifier.cond = sync.NewCond(&notifier.mu)
	return notifier
}

func (n *telegramNotifier) Enqueue(job telegramNotification) {
	n.start()
	if dropped := n.push(job); dropped != nil {
		logger.Warning("telegram notifier queue overflow; dropped event: ", dropped.event)
		n.recordAudit("notifier_overflow", map[string]any{
			"channel":      "telegram",
			"droppedEvent": dropped.event,
			"queuedEvent":  job.event,
		})
	}
}

func (n *telegramNotifier) start() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.started {
		return
	}
	n.started = true
	go n.run()
}

func (n *telegramNotifier) push(job telegramNotification) *telegramNotification {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.queue) >= n.capacity {
		dropped := n.queue[0]
		copy(n.queue, n.queue[1:])
		n.queue[len(n.queue)-1] = job
		n.cond.Signal()
		return &dropped
	}
	n.queue = append(n.queue, job)
	n.cond.Signal()
	return nil
}

func (n *telegramNotifier) next() telegramNotification {
	n.mu.Lock()
	defer n.mu.Unlock()
	for len(n.queue) == 0 {
		n.cond.Wait()
	}
	job := n.queue[0]
	copy(n.queue, n.queue[1:])
	n.queue = n.queue[:len(n.queue)-1]
	return job
}

func (n *telegramNotifier) run() {
	for {
		n.deliver(n.next())
	}
}

func (n *telegramNotifier) deliver(job telegramNotification) {
	attempts := len(n.backoff) + 1
	result := TelegramResult{ErrorClass: "unknown"}
	for attempt := 0; attempt < attempts; attempt++ {
		result = n.send(job.text)
		if result.Success {
			return
		}
		if attempt < len(n.backoff) {
			time.Sleep(n.backoff[attempt])
		}
	}
	if result.ErrorClass == "" {
		result.ErrorClass = "unknown"
	}
	logger.Warning("telegram notification failed: ", result.ErrorClass)
	n.recordAudit("notifier_failed", map[string]any{
		"channel":    "telegram",
		"event":      job.event,
		"errorClass": result.ErrorClass,
		"attempts":   attempts,
	})
}

func (n *telegramNotifier) recordAudit(event string, details map[string]any) {
	if n.audit == nil {
		return
	}
	n.audit(event, details)
}

func getTelegramHTTPClient() *http.Client {
	telegramHTTPClientMu.RLock()
	defer telegramHTTPClientMu.RUnlock()
	return telegramHTTPClient
}

func setTelegramHTTPClient(client *http.Client) func() {
	telegramHTTPClientMu.Lock()
	oldClient := telegramHTTPClient
	telegramHTTPClient = client
	telegramHTTPClientMu.Unlock()
	return func() {
		telegramHTTPClientMu.Lock()
		telegramHTTPClient = oldClient
		telegramHTTPClientMu.Unlock()
	}
}

func (s *TelegramService) TestTelegram() TelegramResult {
	return s.send("S-UI Telegram notification test")
}

func (s *TelegramService) NotifyTelegramEvent(event string, fields map[string]string) {
	enabled, err := s.telegramEnabled()
	if err != nil || !enabled {
		return
	}
	msg := "S-UI event: " + event
	for key, value := range fields {
		if value == "" {
			continue
		}
		msg += "\n" + key + ": " + value
	}
	defaultTelegramNotifier.Enqueue(telegramNotification{event: event, text: msg})
}

func (s *TelegramService) send(text string) TelegramResult {
	enabled, err := s.telegramEnabled()
	if err != nil {
		return TelegramResult{ErrorClass: "settings"}
	}
	if !enabled {
		return TelegramResult{ErrorClass: "disabled"}
	}
	token, err := s.getString("telegramBotToken")
	if err != nil || token == "" {
		return TelegramResult{ErrorClass: "missing_token"}
	}
	chatID, err := s.getString("telegramChatID")
	if err != nil || chatID == "" {
		return TelegramResult{ErrorClass: "missing_chat"}
	}
	payload, err := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return TelegramResult{ErrorClass: "payload"}
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return TelegramResult{ErrorClass: "request"}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := getTelegramHTTPClient().Do(req)
	if err != nil {
		return TelegramResult{ErrorClass: "network"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TelegramResult{ErrorClass: common.NewErrorf("telegram_status_%d", resp.StatusCode).Error()}
	}
	return TelegramResult{Success: true}
}

func (s *TelegramService) telegramEnabled() (bool, error) {
	return s.getBool("telegramEnabled")
}

func recordTelegramNotifierAudit(event string, details map[string]any) {
	if database.GetDB() == nil {
		return
	}
	if err := (&AuditService{}).Record(AuditEvent{
		Event:    event,
		Resource: "notifier",
		Severity: AuditSeverityWarn,
		Details:  details,
	}); err != nil {
		logger.Warning("telegram notifier audit failed:", err)
	}
}
