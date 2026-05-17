package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestGetFinalSubURIFormatsIPv6Hosts(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		settings map[string]string
		want     string
	}{
		{
			name: "loopback with explicit port",
			host: "::1",
			settings: map[string]string{
				"subPort": "8443",
				"subPath": "/sub/",
			},
			want: "http://[::1]:8443/sub/",
		},
		{
			name: "documentation address with explicit port",
			host: "2001:db8::1",
			settings: map[string]string{
				"subPort": "8080",
				"subPath": "/sub/",
			},
			want: "http://[2001:db8::1]:8080/sub/",
		},
		{
			name: "default https port omitted",
			host: "::1",
			settings: map[string]string{
				"subPort":     "443",
				"subCertFile": "/tmp/cert.pem",
				"subKeyFile":  "/tmp/key.pem",
				"subPath":     "/sub/",
			},
			want: "https://[::1]/sub/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settingService := initSettingTestDB(t)
			if _, err := settingService.GetAllSetting(); err != nil {
				t.Fatal(err)
			}
			for key, value := range tt.settings {
				if err := database.GetDB().Model(model.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
					t.Fatal(err)
				}
			}
			uri, err := settingService.GetFinalSubURI(tt.host)
			if err != nil {
				t.Fatal(err)
			}
			if uri != tt.want {
				t.Fatalf("unexpected URI: %s, want %s", uri, tt.want)
			}
		})
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

	validCustomPaths, err := json.Marshal(map[string]string{
		"subJsonPath":  "/json-custom/",
		"subClashPath": "/clash-custom/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validCustomPaths)
	}); err != nil {
		t.Fatalf("valid custom subscription paths rejected: %v", err)
	}

	validFragment, err := json.Marshal(map[string]string{
		"subJsonFragment": `{"enabled":true,"packets":"tlshello"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validFragment)
	}); err != nil {
		t.Fatalf("valid JSON fragment setting rejected: %v", err)
	}
	validNoises, err := json.Marshal(map[string]string{
		"subJsonNoises": `[{"type":"rand","packet":"tlshello"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, validNoises)
	}); err != nil {
		t.Fatalf("valid JSON noises setting rejected: %v", err)
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

	invalidFragment, err := json.Marshal(map[string]string{
		"subJsonFragment": "enabled",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidFragment)
	}); err == nil {
		t.Fatal("expected invalid JSON fragment setting to be rejected")
	}
	invalidNoises, err := json.Marshal(map[string]string{
		"subJsonNoises": `{"type":"rand"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, invalidNoises)
	}); err == nil {
		t.Fatal("expected invalid JSON noises setting to be rejected")
	}

	conflictingPaths, err := json.Marshal(map[string]string{
		"subJsonPath":  "/same/",
		"subClashPath": "/same/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, conflictingPaths)
	}); err == nil {
		t.Fatal("expected duplicate subscription format paths to be rejected")
	}

	subPathConflict, err := json.Marshal(map[string]string{
		"subPath":     "/custom-sub/",
		"subJsonPath": "/custom-sub/json/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		return settingService.Save(tx, subPathConflict)
	}); err == nil {
		t.Fatal("expected subscription format path under subPath to be rejected")
	}
}

func TestSettingSaveDeterministicForDifferentPayloadOrders(t *testing.T) {
	payloads := []json.RawMessage{
		json.RawMessage(`{"telegramNotifyCpu":"true","telegramCpuThreshold":"75","observabilityMemoryCapMB":"64","subPath":"/sub-custom","subJsonPath":"/json-custom","subClashPath":"/clash-custom","subJsonEnable":"false"}`),
		json.RawMessage(`{"subJsonEnable":"false","subClashPath":"/clash-custom","subJsonPath":"/json-custom","subPath":"/sub-custom","observabilityMemoryCapMB":"64","telegramCpuThreshold":"75","telegramNotifyCpu":"true"}`),
	}
	keys := []string{
		"telegramNotifyCpu",
		"telegramCpuThreshold",
		"observabilityMemoryCapMB",
		"subPath",
		"subJsonPath",
		"subClashPath",
		"subJsonEnable",
	}

	var snapshots []map[string]string
	for _, payload := range payloads {
		settingService := initSettingTestDB(t)
		if _, err := settingService.GetAllSetting(); err != nil {
			t.Fatal(err)
		}
		if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
			return settingService.Save(tx, payload)
		}); err != nil {
			t.Fatal(err)
		}
		snapshot := map[string]string{}
		for _, key := range keys {
			var setting model.Setting
			if err := database.GetDB().Where("key = ?", key).First(&setting).Error; err != nil {
				t.Fatal(err)
			}
			snapshot[key] = setting.Value
		}
		snapshots = append(snapshots, snapshot)
	}

	if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
		t.Fatalf("settings differ for equivalent payload orders: %#v != %#v", snapshots[0], snapshots[1])
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

func TestGetTimeLocationRespectsConfiguredLocation(t *testing.T) {
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "timeLocation").Update("value", "UTC").Error; err != nil {
		t.Fatal(err)
	}

	location, err := settingService.GetTimeLocation()
	if err != nil {
		t.Fatal(err)
	}
	if location.String() != "UTC" {
		t.Fatalf("expected configured timeLocation to be respected, got %q", location.String())
	}
}

func TestGetTimeLocationFallsBackToLocalForInvalidLocation(t *testing.T) {
	settingService := initSettingTestDB(t)
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "timeLocation").Update("value", "Invalid/Nowhere").Error; err != nil {
		t.Fatal(err)
	}

	location, err := settingService.GetTimeLocation()
	if err != nil {
		t.Fatal(err)
	}
	if location != time.Local {
		t.Fatalf("expected invalid timeLocation to fall back to time.Local, got %q", location.String())
	}
}
