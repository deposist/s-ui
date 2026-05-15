package service

import (
	"os"
	"strconv"
	"sync"

	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/secretbox"
)

var (
	secretboxFallbackWarning sync.Once

	encryptedSettingKeys = map[string]struct{}{
		"telegramBotToken":      {},
		"telegramProxyPassword": {},
		"telegramProxyURL":      {},
		"telegramProxyUsername": {},
	}
)

func isEncryptedSettingKey(key string) bool {
	_, ok := encryptedSettingKeys[key]
	return ok
}

func writeSecretSettingMarker(settings map[string]string, key string, value string) {
	settings[key+"HasSecret"] = strconv.FormatBool(value != "")
}

func (s *SettingService) getSecretbox() (*secretbox.Box, error) {
	if key := os.Getenv("SUI_SECRETBOX_KEY"); key != "" {
		return secretbox.NewFromString(key)
	}
	secret, err := s.GetSecret()
	if err != nil {
		return nil, err
	}
	secretboxFallbackWarning.Do(func() {
		logger.Warning("SUI_SECRETBOX_KEY is not set; encrypted settings use settings.secret compatibility key")
	})
	return secretbox.New(secret)
}

func (s *SettingService) encryptSettingValue(key string, value string) (string, error) {
	if value == "" || secretbox.IsEncrypted(value) {
		return value, nil
	}
	box, err := s.getSecretbox()
	if err != nil {
		return "", err
	}
	return box.EncryptString(value, key)
}

func (s *SettingService) decryptSettingValue(key string, value string) (string, error) {
	if value == "" || !secretbox.IsEncrypted(value) {
		return value, nil
	}
	box, err := s.getSecretbox()
	if err != nil {
		return "", err
	}
	return box.DecryptString(value, key)
}
