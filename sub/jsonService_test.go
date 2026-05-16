package sub

import (
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
)

func TestJsonServiceAddFragmentToSupportedOutbounds(t *testing.T) {
	initSubTestDB(t)
	settingService := &service.SettingService{}
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "subJsonFragment").Update("value", `{"enabled":true,"packets":"tlshello"}`).Error; err != nil {
		t.Fatal(err)
	}

	outbounds := []map[string]interface{}{
		{"type": "selector", "tag": "proxy"},
		{"type": "vless", "tag": "vless-out"},
		{"type": "vmess", "tag": "vmess-out"},
		{"type": "trojan", "tag": "trojan-out"},
		{"type": "shadowsocks", "tag": "ss-out"},
	}
	config := map[string]interface{}{
		"outbounds": &outbounds,
	}
	if err := (&JsonService{}).addOthers(&config); err != nil {
		t.Fatal(err)
	}

	for _, outbound := range outbounds {
		_, hasFragment := outbound["fragment"]
		switch outbound["type"] {
		case "vless", "vmess", "trojan":
			if !hasFragment {
				t.Fatalf("%s outbound is missing fragment: %#v", outbound["type"], outbound)
			}
		default:
			if hasFragment {
				t.Fatalf("%s outbound should not receive fragment: %#v", outbound["type"], outbound)
			}
		}
	}
}

func TestJsonServiceAddNoisesToSupportedOutbounds(t *testing.T) {
	initSubTestDB(t)
	settingService := &service.SettingService{}
	if _, err := settingService.GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "subJsonNoises").Update("value", `[{"type":"rand","packet":"tlshello"}]`).Error; err != nil {
		t.Fatal(err)
	}

	outbounds := []map[string]interface{}{
		{"type": "selector", "tag": "proxy"},
		{"type": "vless", "tag": "vless-out"},
		{"type": "vmess", "tag": "vmess-out"},
		{"type": "trojan", "tag": "trojan-out"},
		{"type": "shadowsocks", "tag": "ss-out"},
	}
	config := map[string]interface{}{
		"outbounds": &outbounds,
	}
	if err := (&JsonService{}).addOthers(&config); err != nil {
		t.Fatal(err)
	}

	for _, outbound := range outbounds {
		_, hasNoises := outbound["noises"]
		switch outbound["type"] {
		case "vless", "vmess", "trojan":
			if !hasNoises {
				t.Fatalf("%s outbound is missing noises: %#v", outbound["type"], outbound)
			}
		default:
			if hasNoises {
				t.Fatalf("%s outbound should not receive noises: %#v", outbound["type"], outbound)
			}
		}
	}
}
