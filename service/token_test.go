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
