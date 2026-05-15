package service

import (
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"gorm.io/gorm"
)

func (s *ClientService) prepareClientSubSecret(tx *gorm.DB, client *model.Client, preserveExisting bool) error {
	if client.SubSecret != "" {
		return nil
	}
	if preserveExisting && client.Id > 0 {
		var old model.Client
		if err := tx.Model(model.Client{}).Select("sub_secret").Where("id = ?", client.Id).First(&old).Error; err != nil {
			return err
		}
		if old.SubSecret != "" {
			client.SubSecret = old.SubSecret
			return nil
		}
	}
	client.SubSecret = common.Random(32)
	return nil
}
