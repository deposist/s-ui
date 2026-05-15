package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

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

var telegramHTTPClient = &http.Client{Timeout: 10 * time.Second}

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
	result := s.send(msg)
	if !result.Success {
		logger.Warning("telegram notification failed: ", result.ErrorClass)
	}
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
	resp, err := telegramHTTPClient.Do(req)
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
