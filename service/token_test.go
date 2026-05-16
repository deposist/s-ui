package service

import (
	"encoding/json"
	"testing"

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

	for _, scope := range []string{"", "admin", "read", "write", "observability"} {
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
