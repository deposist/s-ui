package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/util/common"
	"github.com/deposist/s-ui-rus-inst/util/secretbox"
	"golang.org/x/crypto/hkdf"
)

var (
	secretboxFallbackWarning sync.Once
	secretboxInvalidWarning  sync.Once
	cookieKeyFallbackWarning sync.Once
	cookieKeyInvalidWarning  sync.Once

	encryptedSettingKeys = map[string]struct{}{
		"telegramBackupPassphrase": {},
		"telegramBotToken":         {},
		"telegramProxyPassword":    {},
		"telegramProxyURL":         {},
		"telegramProxyUsername":    {},
	}
)

const StoredSecretMarker = "••• stored •••"

var (
	cookieKeyHKDFInfo            = []byte("sui:cookie:v1")
	settingsSecretboxKeyHKDFInfo = []byte("sui:settings-secretbox:v1")
	legacyCookieKeyHKDFSalt      = []byte("s-ui session cookie v1")
	legacyCookieKeyHKDFInfo      = []byte("cookie signing key")
)

type secretboxCandidate struct {
	name string
	box  *secretbox.Box
}

func isEncryptedSettingKey(key string) bool {
	_, ok := encryptedSettingKeys[key]
	return ok
}

func writeSecretSettingMarker(settings map[string]string, key string, value string) {
	settings[key+"HasSecret"] = strconv.FormatBool(value != "")
	if key == "telegramBackupPassphrase" {
		if value == "" {
			settings[key] = ""
		} else {
			settings[key] = StoredSecretMarker
		}
	}
}

func (s *SettingService) getSecretbox() (*secretbox.Box, error) {
	candidates, err := s.getSecretboxCandidates()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, common.NewError("no secretbox key candidates")
	}
	return candidates[0].box, nil
}

func (s *SettingService) getSecretboxCandidates() ([]secretboxCandidate, error) {
	if key := strings.TrimSpace(os.Getenv("SUI_SECRETBOX_KEY")); key != "" {
		parsed, err := parseEnvKeyMaterial(key, 32)
		if err == nil {
			box, err := secretbox.NewRawKey(parsed)
			if err != nil {
				return nil, err
			}
			legacyEnvBox, err := secretbox.New(parsed)
			if err != nil {
				return nil, err
			}
			candidates := []secretboxCandidate{
				{name: "env_raw", box: box},
				{name: "legacy_env_hkdf", box: legacyEnvBox},
			}
			settingsSecretCandidates, err := s.settingsSecretboxCandidates()
			if err != nil {
				return nil, err
			}
			return append(candidates, settingsSecretCandidates...), nil
		}
		secretboxInvalidWarning.Do(func() {
			logger.Warning("SUI_SECRETBOX_KEY is invalid:", err, "; encrypted settings use HKDF-derived settings.secret key")
		})
	}
	secretboxFallbackWarning.Do(func() {
		logger.Warning("SUI_SECRETBOX_KEY is not set or invalid; encrypted settings use HKDF-derived settings.secret key")
	})
	return s.settingsSecretboxCandidates()
}

func (s *SettingService) settingsSecretboxCandidates() ([]secretboxCandidate, error) {
	secret, err := s.GetSecret()
	if err != nil {
		return nil, err
	}
	primaryKey, err := deriveHKDFKey(secret, nil, settingsSecretboxKeyHKDFInfo, 32)
	if err != nil {
		return nil, err
	}
	primaryBox, err := secretbox.NewRawKey(primaryKey)
	zeroBytes(primaryKey)
	if err != nil {
		return nil, err
	}
	legacyBox, err := secretbox.New(secret)
	if err != nil {
		return nil, err
	}
	return []secretboxCandidate{
		{name: "settings_secretbox_v1", box: primaryBox},
		{name: "legacy_settings_secret", box: legacyBox},
	}, nil
}

func (s *SettingService) GetCookieKeys() ([][]byte, error) {
	if raw := strings.TrimSpace(os.Getenv("SUI_COOKIE_KEY")); raw != "" {
		keys, err := parseEnvKeyList(raw, 32)
		if err == nil {
			return keys, nil
		}
		cookieKeyInvalidWarning.Do(func() {
			logger.Warning("SUI_COOKIE_KEY is invalid:", err, "; using HKDF-derived compatibility key from settings.secret")
		})
	} else {
		cookieKeyFallbackWarning.Do(func() {
			logger.Warning("SUI_COOKIE_KEY is not set; using HKDF-derived compatibility key from settings.secret")
		})
	}

	secret, err := s.GetSecret()
	if err != nil {
		return nil, err
	}
	cookieKey, err := deriveHKDFKey(secret, nil, cookieKeyHKDFInfo, 32)
	if err != nil {
		return nil, err
	}
	legacyCookieKey, err := deriveHKDFKey(secret, legacyCookieKeyHKDFSalt, legacyCookieKeyHKDFInfo, 32)
	if err != nil {
		zeroBytes(cookieKey)
		return nil, err
	}
	keys := appendUniqueKey(nil, cookieKey)
	keys = appendUniqueKey(keys, legacyCookieKey)
	keys = appendUniqueKey(keys, secret)
	return keys, nil
}

func parseEnvKeyList(raw string, minLen int) ([][]byte, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	keys := make([][]byte, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, err := parseEnvKeyMaterial(part, minLen)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, common.NewError("empty key list")
	}
	return keys, nil
}

func parseEnvKeyMaterial(value string, minLen int) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, common.NewError("empty key material")
	}
	if decoded, ok := decodeKeyMaterial(value); ok {
		if len(decoded) < minLen {
			return nil, common.NewErrorf("decoded key length %d is smaller than %d bytes", len(decoded), minLen)
		}
		return decoded, nil
	}
	return nil, common.NewError("key material must be base64-encoded")
}

func decodeKeyMaterial(value string) ([]byte, bool) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, true
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, true
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, true
	}
	return nil, false
}

func deriveHKDFKey(masterKey []byte, salt []byte, info []byte, keyLen int) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, common.NewError("empty master key")
	}
	if keyLen <= 0 {
		return nil, common.NewError("invalid derived key length")
	}
	key := make([]byte, keyLen)
	reader := hkdf.New(sha256.New, masterKey, salt, info)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func appendUniqueKey(keys [][]byte, key []byte) [][]byte {
	if len(key) == 0 {
		return keys
	}
	for _, existing := range keys {
		if bytes.Equal(existing, key) {
			return keys
		}
	}
	return append(keys, key)
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
	candidates, err := s.getSecretboxCandidates()
	if err != nil {
		return "", err
	}
	for i, candidate := range candidates {
		plaintext, err := candidate.box.DecryptString(value, key)
		if err == nil {
			if i > 0 {
				s.recordSecretboxFallback(key, candidate.name)
			}
			return plaintext, nil
		}
	}
	return "", common.NewError("secret setting decrypt failed")
}

func (s *SettingService) decryptSettingBytes(key string, value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	if !secretbox.IsEncrypted(value) {
		return []byte(value), nil
	}
	candidates, err := s.getSecretboxCandidates()
	if err != nil {
		return nil, err
	}
	for i, candidate := range candidates {
		plaintext, err := candidate.box.DecryptBytes(value, key)
		if err == nil {
			if i > 0 {
				s.recordSecretboxFallback(key, candidate.name)
			}
			return plaintext, nil
		}
	}
	return nil, common.NewError("secret setting decrypt failed")
}

func (s *SettingService) recordSecretboxFallback(key string, candidate string) {
	if database.GetDB() == nil {
		return
	}
	if err := (&AuditService{}).Record(AuditEvent{
		Event:    "settings_secretbox_key_fallback",
		Resource: "settings",
		Severity: AuditSeverityWarn,
		Details: map[string]any{
			"key":       key,
			"candidate": candidate,
		},
	}); err != nil {
		logger.Warning("secretbox fallback audit failed:", err)
	}
}

func zeroBytes(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}
