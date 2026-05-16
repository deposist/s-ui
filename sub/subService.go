package sub

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/deposist/s-ui-rus-inst/util"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

type SubService struct {
	service.SettingService
	LinkService
}

func (s *SubService) GetSubs(subId string) (*string, []string, error) {
	var err error

	client, err := s.getClientBySubId(subId)
	if err != nil {
		return nil, nil, err
	}

	clientInfo := ""
	subShowInfo, _ := s.SettingService.GetSubShowInfo()
	if subShowInfo {
		clientInfo = s.getClientInfo(client)
	}
	subNameInRemark, _ := s.SettingService.GetSubNameInRemark()
	if subNameInRemark {
		clientInfo = " " + client.Name + clientInfo
	}

	linksArray := s.LinkService.GetLinks(&client.Links, "all", clientInfo)
	result := strings.Join(linksArray, "\n")

	headers := s.getClientHeaders(client)

	subEncode, _ := s.SettingService.GetSubEncode()
	if subEncode {
		result = base64.StdEncoding.EncodeToString([]byte(result))
	}

	return &result, headers, nil
}

func (j *SubService) getClientBySubId(subId string) (*model.Client, error) {
	db := database.GetDB()
	client := &model.Client{}
	err := db.Model(model.Client{}).Where("enable = true and sub_secret = ?", subId).First(client).Error
	if err == nil {
		return client, j.ensureClientSubSecret(db, client)
	}
	if err != nil && !database.IsNotFound(err) {
		return nil, err
	}
	required, _ := j.SettingService.GetSubSecretRequired()
	if required {
		return nil, gorm.ErrRecordNotFound
	}
	err = db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, err
	}
	return client, j.ensureClientSubSecret(db, client)
}

func (s *SubService) getClientHeaders(client *model.Client) []string {
	updateInterval, _ := s.SettingService.GetSubUpdates()
	headers := util.GetHeaders(client, updateInterval)
	if title, err := s.SettingService.GetSubTitle(); err == nil && title != "" {
		headers[2] = title
	}
	supportURL, _ := s.SettingService.GetSubSupportUrl()
	profileURL, _ := s.SettingService.GetSubProfileUrl()
	announce, _ := s.SettingService.GetSubAnnounce()
	headers = append(headers, supportURL, profileURL, announce)
	return headers
}

func (s *SubService) getClientInfo(c *model.Client) string {
	now := time.Now().Unix()

	var result []string
	if vol := c.Volume - (c.Up + c.Down); vol > 0 {
		result = append(result, fmt.Sprintf("%s%s", s.formatTraffic(vol), "📊"))
	}
	if c.Expiry > 0 {
		result = append(result, fmt.Sprintf("%d%s⏳", (c.Expiry-now)/86400, "Days"))
	}
	if len(result) > 0 {
		return " " + strings.Join(result, " ")
	} else {
		return " ♾"
	}
}

func (s *SubService) formatTraffic(trafficBytes int64) string {
	if trafficBytes < 1024 {
		return fmt.Sprintf("%.2fB", float64(trafficBytes)/float64(1))
	} else if trafficBytes < (1024 * 1024) {
		return fmt.Sprintf("%.2fKB", float64(trafficBytes)/float64(1024))
	} else if trafficBytes < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fMB", float64(trafficBytes)/float64(1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fGB", float64(trafficBytes)/float64(1024*1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fTB", float64(trafficBytes)/float64(1024*1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2fEB", float64(trafficBytes)/float64(1024*1024*1024*1024*1024))
	}
}

func (s *SubService) ensureClientSubSecret(db *gorm.DB, client *model.Client) error {
	if client.SubSecret != "" {
		return nil
	}
	secret, err := uuid.NewV4()
	if err != nil {
		return err
	}
	client.SubSecret = secret.String()
	return db.Model(model.Client{}).Where("id = ?", client.Id).Update("sub_secret", client.SubSecret).Error
}
