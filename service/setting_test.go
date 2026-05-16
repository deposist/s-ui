package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"gorm.io/gorm"
)

func TestGetFinalSubURIOmitsDefaultPorts(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB("file::memory:?cache=shared"); err != nil {
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

	settingService := &SettingService{}
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	settings := map[string]string{
		"subPort":     "443",
		"subCertFile": "/tmp/cert.pem",
		"subKeyFile":  "/tmp/key.pem",
		"subPath":     "/sub/",
	}
	for key, value := range settings {
		if err := db.Model(model.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	uri, err := settingService.GetFinalSubURI("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if uri != "https://example.com/sub/" {
		t.Fatalf("unexpected URI: %s", uri)
	}
}

func TestSaveRejectsReservedWebPath(t *testing.T) {
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{
		"webPath": "/api/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, payload)
	}); err == nil {
		t.Fatal("expected reserved webPath to be rejected")
	}
}

func TestSaveAllowsDefaultSubPathButRejectsOtherReservedSubPaths(t *testing.T) {
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	validPayload, err := json.Marshal(map[string]string{
		"subPath": "/sub/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validPayload)
	}); err != nil {
		t.Fatalf("default subPath should remain valid: %v", err)
	}

	invalidPayload, err := json.Marshal(map[string]string{
		"subPath": "/json/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidPayload)
	}); err == nil {
		t.Fatal("expected reserved subPath to be rejected")
	}
}

func TestSubscriptionSettingsDefaultsAndValidation(t *testing.T) {
	settingService := initSettingTestDB(t)
	settings, err := settingService.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"subLinkEnable",
		"subJsonEnable",
		"subClashEnable",
		"subJsonPath",
		"subClashPath",
		"subJsonURI",
		"subClashURI",
		"subTitle",
		"subSupportUrl",
		"subProfileUrl",
		"subAnnounce",
		"subNameInRemark",
		"subJsonFragment",
		"subJsonNoises",
		"subJsonMux",
		"subJsonDirectRules",
		"subRateLimitPerIP",
	} {
		if _, ok := (*settings)[key]; !ok {
			t.Fatalf("missing default setting %s", key)
		}
	}

	validPayload, err := json.Marshal(map[string]string{
		"subJsonPath":       "/json/",
		"subClashPath":      "/clash/",
		"subSupportUrl":     "https://example.com/support",
		"subProfileUrl":     "https://example.com/profile",
		"subJsonEnable":     "false",
		"subRateLimitPerIP": "120",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validPayload)
	}); err != nil {
		t.Fatalf("valid subscription settings rejected: %v", err)
	}

	invalidPayload, err := json.Marshal(map[string]string{
		"subJsonEnable": "sometimes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidPayload)
	}); err == nil {
		t.Fatal("expected invalid boolean setting to be rejected")
	}

	invalidURLPayload, err := json.Marshal(map[string]string{
		"subSupportUrl": "ftp://example.com/support",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidURLPayload)
	}); err == nil {
		t.Fatal("expected invalid URL setting to be rejected")
	}
}

func TestSaveValidatesTelegramProxyURLBeforeEncrypting(t *testing.T) {
	t.Setenv("SUI_SECRETBOX_KEY", "test-secretbox-key")
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}

	invalidPayload, err := json.Marshal(map[string]string{
		"telegramProxyURL": "http://127.0.0.1:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidPayload)
	}); err == nil {
		t.Fatal("expected private telegramProxyURL to be rejected")
	}

	validPayload, err := json.Marshal(map[string]string{
		"telegramProxyURL": "socks5://8.8.8.8:1080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validPayload)
	}); err != nil {
		t.Fatalf("expected public telegramProxyURL to be accepted: %v", err)
	}
	decrypted, err := settingService.getString("telegramProxyURL")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "socks5://8.8.8.8:1080" {
		t.Fatalf("unexpected stored telegramProxyURL: %q", decrypted)
	}
}

func TestSaveValidatesTelegramCPUSettings(t *testing.T) {
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}

	validPayload, err := json.Marshal(map[string]string{
		"telegramNotifyCpu":    "true",
		"telegramCpuThreshold": "85",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validPayload)
	}); err != nil {
		t.Fatalf("valid CPU settings rejected: %v", err)
	}

	invalidPayload, err := json.Marshal(map[string]string{
		"telegramCpuThreshold": "101",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidPayload)
	}); err == nil {
		t.Fatal("expected invalid CPU threshold to be rejected")
	}
}
