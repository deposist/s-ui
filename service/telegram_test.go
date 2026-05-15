package service

import (
	"net/http"
	"testing"
	"time"
)

type countingRoundTripper struct {
	count int
}

func (r *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	r.count++
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     http.Header{},
	}, nil
}

func TestTelegramDisabledMakesNoOutboundCall(t *testing.T) {
	initSettingTestDB(t)
	rt := &countingRoundTripper{}
	oldClient := telegramHTTPClient
	telegramHTTPClient = &http.Client{Transport: rt, Timeout: time.Second}
	t.Cleanup(func() { telegramHTTPClient = oldClient })

	result := (&TelegramService{}).TestTelegram()
	if result.Success || result.ErrorClass != "disabled" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if rt.count != 0 {
		t.Fatalf("disabled telegram made %d outbound calls", rt.count)
	}
}
