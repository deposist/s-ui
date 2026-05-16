package service

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/realtime"
	"github.com/deposist/s-ui-rus-inst/util"
	"github.com/deposist/s-ui-rus-inst/util/common"

	"gorm.io/gorm"
)

var defaultConfig = `{
  "log": {
    "level": "info"
  },
  "dns": {
    "servers": [],
    "rules": []
  },
  "route": {
    "rules": [
		  {
        "action": "sniff"
      },
      {
        "protocol": [
          "dns"
        ],
        "action": "hijack-dns"
      }
    ]
  },
  "experimental": {}
}`

var defaultValueMap = map[string]string{
	"webListen":              "",
	"webDomain":              "",
	"webPort":                "2095",
	"secret":                 common.Random(32),
	"installSalt":            common.Random(32),
	"webCertFile":            "",
	"webKeyFile":             "",
	"webPath":                "/app/",
	"webURI":                 "",
	"sessionMaxAge":          "0",
	"sessionGeneration":      "",
	"trafficAge":             "30",
	"timeLocation":           "Europe/Moscow",
	"subListen":              "",
	"subPort":                "2096",
	"subPath":                "/sub/",
	"subDomain":              "",
	"subCertFile":            "",
	"subKeyFile":             "",
	"subUpdates":             "12",
	"subEncode":              "true",
	"subShowInfo":            "false",
	"subSecretRequired":      "false",
	"subRateLimitPerIP":      "60",
	"subLinkEnable":          "true",
	"subJsonEnable":          "true",
	"subClashEnable":         "true",
	"subJsonPath":            "/json/",
	"subClashPath":           "/clash/",
	"subJsonURI":             "",
	"subClashURI":            "",
	"subTitle":               "",
	"subSupportUrl":          "",
	"subProfileUrl":          "",
	"subAnnounce":            "",
	"subNameInRemark":        "false",
	"subJsonFragment":        "",
	"subJsonNoises":          "",
	"subJsonMux":             "false",
	"subJsonDirectRules":     "false",
	"subURI":                 "",
	"subJsonExt":             "",
	"subClashExt":            "",
	"auditRetentionDays":     "30",
	"ipShowRaw":              "false",
	"ipHistoryRetentionDays": "30",
	"telegramEnabled":        "false",
	"telegramBotToken":       "",
	"telegramChatID":         "",
	"telegramProxyURL":       "",
	"telegramProxyUsername":  "",
	"telegramProxyPassword":  "",
	"telegramCpuThreshold":   "90",
	"telegramNotifyCpu":      "false",
	"telegramReport":         "false",
	"telegramReportCron":     "",
	"config":                 defaultConfig,
	"version":                config.GetVersion(),
}

type SettingService struct {
}

func (s *SettingService) GetAllSetting() (*map[string]string, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := map[string]string{}
	existingKeys := map[string]bool{}

	for _, setting := range settings {
		existingKeys[setting.Key] = true
		if isEncryptedSettingKey(setting.Key) {
			writeSecretSettingMarker(allSetting, setting.Key, setting.Value)
			continue
		}
		allSetting[setting.Key] = setting.Value
	}

	for key, defaultValue := range defaultValueMap {
		if !existingKeys[key] {
			err = s.saveSetting(key, defaultValue)
			if err != nil {
				return nil, err
			}
			if isEncryptedSettingKey(key) {
				writeSecretSettingMarker(allSetting, key, defaultValue)
			} else {
				allSetting[key] = defaultValue
			}
		}
	}

	// Due to security principles
	delete(allSetting, "secret")
	delete(allSetting, "installSalt")
	delete(allSetting, "sessionGeneration")
	delete(allSetting, "config")
	delete(allSetting, "version")

	return &allSetting, nil
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.Setting{}).Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	if isEncryptedSettingKey(key) {
		return s.decryptSettingValue(key, setting.Value)
	}
	return setting.Value, nil
}

func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getBool(key string) (bool, error) {
	str, err := s.getString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

// func (s *SettingService) setBool(key string, value bool) error {
// 	return s.setString(key, strconv.FormatBool(value))
// }

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}
func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

func (s *SettingService) SetPort(port int) error {
	return s.setInt("webPort", port)
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) GetWebPath() (string, error) {
	webPath, err := s.getString("webPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return webPath, nil
}

func (s *SettingService) SetWebPath(webPath string) error {
	webPath, err := normalizeAndValidatePathSetting("webPath", webPath)
	if err != nil {
		return err
	}
	return s.setString("webPath", webPath)
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		err := s.saveSetting("secret", secret)
		if err != nil {
			logger.Warning("save secret failed:", err)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) GetInstallSalt() ([]byte, error) {
	salt, err := s.getString("installSalt")
	if salt == defaultValueMap["installSalt"] {
		if saveErr := s.saveSetting("installSalt", salt); saveErr != nil {
			logger.Warning("save install salt failed:", saveErr)
		}
	}
	return []byte(salt), err
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetSessionGeneration() (string, error) {
	return s.getString("sessionGeneration")
}

func (s *SettingService) RotateSessionGeneration() (string, error) {
	generation := common.Random(32)
	if err := s.setString("sessionGeneration", generation); err != nil {
		return generation, err
	}
	realtime.CloseAll("session_rotated")
	return generation, nil
}

func (s *SettingService) GetTrafficAge() (int, error) {
	return s.getInt("trafficAge")
}

func (s *SettingService) GetAuditRetentionDays() (int, error) {
	return s.getInt("auditRetentionDays")
}

func (s *SettingService) GetIPHistoryRetentionDays() (int, error) {
	return s.getInt("ipHistoryRetentionDays")
}

func (s *SettingService) GetIPShowRaw() (bool, error) {
	return s.getBool("ipShowRaw")
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	l, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		l = "Local"
	}
	location, err := time.LoadLocation(l)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", l, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) GetSubListen() (string, error) {
	return s.getString("subListen")
}

func (s *SettingService) GetSubPort() (int, error) {
	return s.getInt("subPort")
}

func (s *SettingService) SetSubPort(subPort int) error {
	return s.setInt("subPort", subPort)
}

func (s *SettingService) GetSubPath() (string, error) {
	subPath, err := s.getString("subPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	return subPath, nil
}

func (s *SettingService) SetSubPath(subPath string) error {
	subPath, err := normalizeAndValidatePathSetting("subPath", subPath)
	if err != nil {
		return err
	}
	return s.setString("subPath", subPath)
}

func (s *SettingService) GetSubDomain() (string, error) {
	return s.getString("subDomain")
}

func (s *SettingService) GetSubCertFile() (string, error) {
	return s.getString("subCertFile")
}

func (s *SettingService) GetSubKeyFile() (string, error) {
	return s.getString("subKeyFile")
}

func (s *SettingService) GetSubUpdates() (int, error) {
	return s.getInt("subUpdates")
}

func (s *SettingService) GetSubEncode() (bool, error) {
	return s.getBool("subEncode")
}

func (s *SettingService) GetSubShowInfo() (bool, error) {
	return s.getBool("subShowInfo")
}

func (s *SettingService) GetSubSecretRequired() (bool, error) {
	return s.getBool("subSecretRequired")
}

func (s *SettingService) GetSubRateLimitPerIP() (int, error) {
	return s.getInt("subRateLimitPerIP")
}

func (s *SettingService) GetSubLinkEnable() (bool, error) {
	return s.getBool("subLinkEnable")
}

func (s *SettingService) GetSubJsonEnable() (bool, error) {
	return s.getBool("subJsonEnable")
}

func (s *SettingService) GetSubClashEnable() (bool, error) {
	return s.getBool("subClashEnable")
}

func (s *SettingService) GetSubJsonPath() (string, error) {
	subJsonPath, err := s.getString("subJsonPath")
	if err != nil {
		return "", err
	}
	return normalizeURLPath(subJsonPath), nil
}

func (s *SettingService) GetSubClashPath() (string, error) {
	subClashPath, err := s.getString("subClashPath")
	if err != nil {
		return "", err
	}
	return normalizeURLPath(subClashPath), nil
}

func (s *SettingService) GetSubJsonURI() (string, error) {
	return s.getString("subJsonURI")
}

func (s *SettingService) GetSubClashURI() (string, error) {
	return s.getString("subClashURI")
}

func (s *SettingService) GetSubTitle() (string, error) {
	return s.getString("subTitle")
}

func (s *SettingService) GetSubSupportUrl() (string, error) {
	return s.getString("subSupportUrl")
}

func (s *SettingService) GetSubProfileUrl() (string, error) {
	return s.getString("subProfileUrl")
}

func (s *SettingService) GetSubAnnounce() (string, error) {
	return s.getString("subAnnounce")
}

func (s *SettingService) GetSubNameInRemark() (bool, error) {
	return s.getBool("subNameInRemark")
}

func (s *SettingService) GetSubJsonFragment() (string, error) {
	return s.getString("subJsonFragment")
}

func (s *SettingService) GetSubJsonNoises() (string, error) {
	return s.getString("subJsonNoises")
}

func (s *SettingService) GetSubJsonMux() (bool, error) {
	return s.getBool("subJsonMux")
}

func (s *SettingService) GetSubJsonDirectRules() (bool, error) {
	return s.getBool("subJsonDirectRules")
}

func (s *SettingService) GetSubURI() (string, error) {
	return s.getString("subURI")
}

func (s *SettingService) GetFinalSubURI(host string) (string, error) {
	allSetting, err := s.GetAllSetting()
	if err != nil {
		return "", err
	}
	SubURI := (*allSetting)["subURI"]
	if SubURI != "" {
		return SubURI, nil
	}
	protocol := "http"
	if (*allSetting)["subKeyFile"] != "" && (*allSetting)["subCertFile"] != "" {
		protocol = "https"
	}
	if (*allSetting)["subDomain"] != "" {
		host = (*allSetting)["subDomain"]
	}
	portValue := (*allSetting)["subPort"]
	port := ":" + portValue
	if (portValue == "80" && protocol == "http") || (portValue == "443" && protocol == "https") {
		port = ""
	}
	return protocol + "://" + host + port + (*allSetting)["subPath"], nil
}

func (s *SettingService) GetConfig() (string, error) {
	return s.getString("config")
}

func (s *SettingService) SetConfig(config string) error {
	return s.setString("config", config)
}

func (s *SettingService) SaveConfig(tx *gorm.DB, config json.RawMessage) error {
	configs, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return tx.Model(model.Setting{}).Where("key = ?", "config").Update("value", string(configs)).Error
}

func (s *SettingService) Save(tx *gorm.DB, data json.RawMessage) error {
	var err error
	var settings map[string]string
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	for key, obj := range settings {
		if strings.HasSuffix(key, "HasSecret") {
			continue
		}
		if key == "telegramProxyURL" && obj != "" {
			if err = validateTelegramProxyURL(obj); err != nil {
				return err
			}
		}
		if err = validateTelegramSettingInput(key, obj); err != nil {
			return err
		}
		if err = validateSubscriptionSettingInput(key, obj); err != nil {
			return err
		}
		if isEncryptedSettingKey(key) {
			if obj == "" {
				continue
			}
			obj, err = s.encryptSettingValue(key, obj)
			if err != nil {
				return err
			}
		}
		// Secure file existence check
		if obj != "" && (key == "webCertFile" ||
			key == "webKeyFile" ||
			key == "subCertFile" ||
			key == "subKeyFile") {
			err = s.fileExists(obj)
			if err != nil {
				return common.NewError(" -> ", obj, " is not exists")
			}
		}

		// Correct Pathes start and ends with `/`
		if key == "webPath" ||
			key == "subPath" ||
			key == "subJsonPath" ||
			key == "subClashPath" {
			obj, err = normalizeAndValidatePathSetting(key, obj)
			if err != nil {
				return err
			}
		}

		// Delete all stats if it is set to 0
		if key == "trafficAge" && obj == "0" {
			err = tx.Where("id > 0").Delete(model.Stats{}).Error
			if err != nil {
				return err
			}
		}
		err = tx.Model(model.Setting{}).Where("key = ?", key).Update("value", obj).Error
		if err != nil {
			return err
		}
	}
	return err
}

func (s *SettingService) GetSubJsonExt() (string, error) {
	return s.getString("subJsonExt")
}

func (s *SettingService) GetSubClashExt() (string, error) {
	return s.getString("subClashExt")
}

func (s *SettingService) GetTelegramCpuThreshold() (int, error) {
	return s.getInt("telegramCpuThreshold")
}

func (s *SettingService) GetTelegramNotifyCpu() (bool, error) {
	return s.getBool("telegramNotifyCpu")
}

func (s *SettingService) GetTelegramReport() (bool, error) {
	return s.getBool("telegramReport")
}

func (s *SettingService) GetTelegramReportCron() (string, error) {
	return s.getString("telegramReportCron")
}

func (s *SettingService) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}

func normalizeAndValidatePathSetting(key string, path string) (string, error) {
	path = normalizeURLPath(path)
	if err := util.ValidatePath(path, reservedPathPrefixesForSetting(key)); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeURLPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func reservedPathPrefixesForSetting(key string) []string {
	ownPrefix := ""
	switch key {
	case "subPath":
		ownPrefix = "/sub/"
	case "subJsonPath":
		ownPrefix = "/json/"
	case "subClashPath":
		ownPrefix = "/clash/"
	}
	if ownPrefix == "" {
		return util.ReservedPathPrefixes
	}
	reserved := make([]string, 0, len(util.ReservedPathPrefixes))
	for _, prefix := range util.ReservedPathPrefixes {
		if prefix == ownPrefix {
			continue
		}
		reserved = append(reserved, prefix)
	}
	return reserved
}

func validateSubscriptionSettingInput(key string, value string) error {
	switch key {
	case "subLinkEnable", "subJsonEnable", "subClashEnable", "subNameInRemark", "subJsonMux", "subJsonDirectRules":
		if _, err := strconv.ParseBool(value); err != nil {
			return common.NewError("invalid boolean setting: ", key)
		}
	case "subRateLimitPerIP":
		limit, err := strconv.Atoi(value)
		if err != nil || limit <= 0 || limit > 10000 {
			return common.NewError("invalid rate-limit setting: ", key)
		}
	case "subJsonURI", "subClashURI", "subSupportUrl", "subProfileUrl":
		if err := validateOptionalHTTPURL(value); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalHTTPURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return common.NewError("invalid URL setting")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return common.NewError("invalid URL setting")
	}
	if parsed.User != nil {
		return common.NewError("invalid URL setting")
	}
	return nil
}

func validateTelegramSettingInput(key string, value string) error {
	switch key {
	case "telegramNotifyCpu", "telegramReport":
		if _, err := strconv.ParseBool(value); err != nil {
			return common.NewError("invalid boolean setting: ", key)
		}
	case "telegramCpuThreshold":
		threshold, err := strconv.Atoi(value)
		if err != nil || threshold <= 0 || threshold > 100 {
			return common.NewError("invalid cpu threshold setting")
		}
	case "telegramReportCron":
		if _, err := ParseTelegramReportCron(value); err != nil {
			return err
		}
	}
	return nil
}
