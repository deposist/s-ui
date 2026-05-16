package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
)

func initSubTestDB(t *testing.T) {
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
}

func TestGetClientBySubIdPrefersSecretAndSupportsLegacyName(t *testing.T) {
	initSubTestDB(t)
	if _, err := (&service.SettingService{}).GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable:    true,
		Name:      "legacy-name",
		SubSecret: "secret-id",
		Inbounds:  []byte("[]"),
		Links:     []byte("[]"),
	}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	subService := &SubService{}
	bySecret, err := subService.getClientBySubId("secret-id")
	if err != nil {
		t.Fatal(err)
	}
	if bySecret.Name != "legacy-name" {
		t.Fatalf("unexpected secret lookup client: %#v", bySecret)
	}

	byName, err := subService.getClientBySubId("legacy-name")
	if err != nil {
		t.Fatal(err)
	}
	if byName.SubSecret != "secret-id" {
		t.Fatalf("legacy lookup did not return expected client: %#v", byName)
	}
}

func TestGetClientBySubIdCanDisableLegacyName(t *testing.T) {
	initSubTestDB(t)
	settingService := &service.SettingService{}
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "subSecretRequired").Update("value", "true").Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{
		Enable:    true,
		Name:      "legacy-name",
		SubSecret: "secret-id",
		Inbounds:  []byte("[]"),
		Links:     []byte("[]"),
	}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	subService := &SubService{}
	if _, err := subService.getClientBySubId("legacy-name"); err == nil {
		t.Fatal("legacy name lookup should be disabled when subSecretRequired=true")
	}
	if _, err := subService.getClientBySubId("secret-id"); err != nil {
		t.Fatalf("secret lookup should still work: %v", err)
	}
}

func TestSafeSubscriptionHeadersRemovesControlCharacters(t *testing.T) {
	got := safeSubscriptionHeaders([]string{"ok\r\nInjected: bad"})[0]
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("header was not sanitized: %q", got)
	}
}
