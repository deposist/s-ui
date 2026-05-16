package service

import (
	"strconv"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

func TestRotateSubSecretChangesExistingClientSecret(t *testing.T) {
	initSettingTestDB(t)
	client := model.Client{
		Enable:    true,
		Name:      "alice",
		SubSecret: "old-secret",
		Inbounds:  []byte("[]"),
		Links:     []byte("[]"),
	}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	name, err := (&ClientService{}).RotateSubSecret(strconv.FormatUint(uint64(client.Id), 10))
	if err != nil {
		t.Fatal(err)
	}
	if name != "alice" {
		t.Fatalf("unexpected client name: %s", name)
	}

	var stored model.Client
	if err := database.GetDB().Where("id = ?", client.Id).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SubSecret == "" || stored.SubSecret == "old-secret" {
		t.Fatalf("sub secret was not rotated: %#v", stored)
	}
}
