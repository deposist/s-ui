package service

import (
	"strings"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
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
