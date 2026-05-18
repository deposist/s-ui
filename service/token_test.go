package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

func TestLoadTokensMigratesLegacyPlaintextToken(t *testing.T) {
	initSettingTestDB(t)
	userService := &UserService{}

	if err := database.GetDB().Create(&model.Tokens{
		Desc:   "legacy",
		Token:  "legacy-token",
		Expiry: 0,
		UserId: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	raw, err := userService.LoadTokens()
	if err != nil {
		t.Fatal(err)
	}
	var loaded []map[string]any
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one loaded token, got %d", len(loaded))
	}
	if loaded[0]["tokenHash"] == "" || loaded[0]["token"] != nil {
		t.Fatalf("loaded token leaked plaintext or missed hash: %#v", loaded[0])
	}

	var stored model.Tokens
	if err := database.GetDB().First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Token != "" {
		t.Fatalf("legacy plaintext token was not cleared: %q", stored.Token)
	}
	if stored.TokenHash == "" || stored.TokenPrefix != tokenPrefix("legacy-token") {
		t.Fatalf("legacy token hash/prefix not populated: %#v", stored)
	}
	if !stored.Enabled || stored.Scope != defaultAPITokenScope {
		t.Fatalf("legacy token defaults not populated: %#v", stored)
	}
}

func TestAddTokenValidatesScopeAllowlist(t *testing.T) {
	initSettingTestDB(t)
	userService := &UserService{}

	for _, scope := range []string{"", "admin", "read", "write", "database", "telegram", "observability", "xui_remote"} {
		if _, err := userService.AddToken("admin", 0, "valid "+scope, scope); err != nil {
			t.Fatalf("scope %q should be accepted: %v", scope, err)
		}
	}
	if _, err := userService.AddToken("admin", 0, "invalid", "full"); err == nil {
		t.Fatal("scope full should be rejected")
	}
	if _, err := userService.AddToken("admin", 0, "invalid", "admin "); err != nil {
		t.Fatalf("trimmed admin scope should be accepted: %v", err)
	}

	var tokens []model.Tokens
	if err := database.GetDB().Order("id asc").Find(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	for _, token := range tokens {
		if !apiTokenScopeAllowed(token.Scope) {
			t.Fatalf("stored invalid scope: %#v", token)
		}
	}
}

func TestTokenUseDebouncerKeepsLatestUseUntilFlush(t *testing.T) {
	written := make(chan map[uint]tokenUseUpdate, 1)
	debouncer := newTokenUseDebouncer(time.Hour, func(updates map[uint]tokenUseUpdate) error {
		copied := make(map[uint]tokenUseUpdate, len(updates))
		for id, update := range updates {
			copied[id] = update
		}
		written <- copied
		return nil
	})

	debouncer.Record(7, "198.51.100.1", 100)
	debouncer.Record(7, "198.51.100.2", 200)
	if err := debouncer.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}

	updates := <-written
	if len(updates) != 1 {
		t.Fatalf("expected one debounced token update, got %#v", updates)
	}
	if updates[7].ip != "198.51.100.2" || updates[7].ts != 200 {
		t.Fatalf("debouncer did not keep latest token use: %#v", updates[7])
	}
}

func TestRecordTokenUseFlushesBatchedUpdate(t *testing.T) {
	initSettingTestDB(t)
	resetTokenUseDebouncerForTest()
	t.Cleanup(resetTokenUseDebouncerForTest)

	userService := &UserService{}
	if err := database.GetDB().Create(&model.Tokens{
		Desc:      "tracked",
		TokenHash: "hash",
		Enabled:   true,
		UserId:    1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var token model.Tokens
	if err := database.GetDB().Where("desc = ?", "tracked").First(&token).Error; err != nil {
		t.Fatal(err)
	}

	if err := userService.RecordTokenUse(token.Id, "198.51.100.10"); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().First(&token, token.Id).Error; err != nil {
		t.Fatal(err)
	}
	if token.LastUsedAt != 0 || token.LastUsedIP != "" {
		t.Fatalf("token use was written before flush: %#v", token)
	}

	if err := userService.RecordTokenUse(token.Id, "198.51.100.11"); err != nil {
		t.Fatal(err)
	}
	if err := StopTokenUseDebouncer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().First(&token, token.Id).Error; err != nil {
		t.Fatal(err)
	}
	if token.LastUsedAt == 0 || token.LastUsedIP != "198.51.100.11" {
		t.Fatalf("token use was not flushed with latest value: %#v", token)
	}
}
