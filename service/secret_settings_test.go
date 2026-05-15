package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/util/secretbox"
	"gorm.io/gorm"
)

func initSettingTestDB(t *testing.T) *SettingService {
	t.Helper()
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(filepath.Join(t.TempDir(), "s-ui.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if d := database.GetDB(); d != nil {
			if sqlDB, err := d.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})
	return &SettingService{}
}

func TestSecretSettingIsEncryptedAndMasked(t *testing.T) {
	t.Setenv("SUI_SECRETBOX_KEY", "test-secretbox-key")
	settingService := initSettingTestDB(t)

	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]string{
		"telegramBotToken": "123456:secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, payload)
	}); err != nil {
		t.Fatal(err)
	}

	var setting model.Setting
	if err := database.GetDB().Where("key = ?", "telegramBotToken").First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if setting.Value == "123456:secret-token" || !secretbox.IsEncrypted(setting.Value) {
		t.Fatalf("secret setting was not encrypted: %q", setting.Value)
	}

	decrypted, err := settingService.getString("telegramBotToken")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "123456:secret-token" {
		t.Fatalf("unexpected decrypted value %q", decrypted)
	}

	settings, err := settingService.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := (*settings)["telegramBotToken"]; ok {
		t.Fatal("raw telegramBotToken leaked through settings API")
	}
	if (*settings)["telegramBotTokenHasSecret"] != "true" {
		t.Fatalf("expected has-secret marker, got %q", (*settings)["telegramBotTokenHasSecret"])
	}

	emptyPayload, err := json.Marshal(map[string]string{
		"telegramBotToken": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, emptyPayload)
	}); err != nil {
		t.Fatal(err)
	}
	afterEmpty, err := settingService.getString("telegramBotToken")
	if err != nil {
		t.Fatal(err)
	}
	if afterEmpty != "123456:secret-token" {
		t.Fatalf("empty secret save should keep old value, got %q", afterEmpty)
	}
}
