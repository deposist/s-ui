package importxui

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
	"golang.org/x/crypto/hkdf"
	"gorm.io/gorm"
)

const profileKeySalt = "xui-sync-profile-v1"

type SyncProfileSource struct {
	Type               string `json:"type"`
	URL                string `json:"url,omitempty"`
	Host               string `json:"host,omitempty"`
	Port               int    `json:"port,omitempty"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	KeyPath            string `json:"keyPath,omitempty"`
	RemotePath         string `json:"remotePath,omitempty"`
	BaseURL            string `json:"baseUrl,omitempty"`
	ConfirmHostKey     bool   `json:"confirmHostKey,omitempty"`
	HostKeyFingerprint string `json:"hostKeyFingerprint,omitempty"`
}

type SyncProfileInput struct {
	Name       string            `json:"name"`
	SourceType string            `json:"sourceType"`
	Source     SyncProfileSource `json:"source"`
	Strategy   Strategy          `json:"strategy"`
	OnlyNew    bool              `json:"onlyNew"`
	Enabled    bool              `json:"enabled"`
	Schedule   string            `json:"schedule"`
}

func SaveSyncProfile(input SyncProfileInput) (*model.XUISyncProfile, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("missing profile name")
	}
	if input.SourceType == "" {
		input.SourceType = input.Source.Type
	}
	if input.Strategy == "" {
		input.Strategy = StrategyMerge
	}
	if err := input.Strategy.Validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input.Source)
	if err != nil {
		return nil, err
	}
	ciphertext, salt, err := EncryptProfileSource(raw)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	profile := &model.XUISyncProfile{
		Name:       input.Name,
		SourceType: input.SourceType,
		SourceJSON: ciphertext,
		SourceSalt: salt,
		Strategy:   string(input.Strategy),
		OnlyNew:    input.OnlyNew,
		Enabled:    input.Enabled,
		Schedule:   input.Schedule,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if profile.Schedule == "" {
		profile.Schedule = "0 */6 * * *"
	}
	if db := database.GetDB(); db != nil {
		var existing model.XUISyncProfile
		err := db.Where("name = ?", profile.Name).First(&existing).Error
		if err == nil {
			profile.Id = existing.Id
			profile.CreatedAt = existing.CreatedAt
			return profile, db.Save(profile).Error
		}
		if err != nil && !database.IsNotFound(err) {
			return nil, err
		}
		return profile, db.Create(profile).Error
	}
	return nil, fmt.Errorf("destination database is not initialized")
}

func LoadSyncProfileSource(profile model.XUISyncProfile) (SyncProfileSource, error) {
	raw, err := DecryptProfileSource(profile.SourceJSON, profile.SourceSalt)
	if err != nil {
		return SyncProfileSource{}, err
	}
	var source SyncProfileSource
	if err := json.Unmarshal(raw, &source); err != nil {
		return SyncProfileSource{}, err
	}
	return source, nil
}

func EncryptProfileSource(plaintext []byte) ([]byte, []byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	key, err := profileEncryptionKey(salt)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext := append([]byte{}, nonce...)
	ciphertext = gcm.Seal(ciphertext, nonce, plaintext, nil)
	return ciphertext, salt, nil
}

func DecryptProfileSource(ciphertext []byte, salt []byte) ([]byte, error) {
	key, err := profileEncryptionKey(salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("profile ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	payload := ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}

func profileEncryptionKey(rowSalt []byte) ([]byte, error) {
	if keyFile := strings.TrimSpace(os.Getenv("XUI_PROFILE_KEY_FILE")); keyFile != "" {
		raw, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}
		key := bytesOrBase64Key(raw)
		if len(key) != 32 {
			return nil, fmt.Errorf("XUI_PROFILE_KEY_FILE must contain 32 bytes raw or base64")
		}
		return key, nil
	}
	seed := []byte(config.GetSecret())
	salt := append([]byte(profileKeySalt+":"), rowSalt...)
	reader := hkdf.New(sha256.New, seed, salt, nil)
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func bytesOrBase64Key(raw []byte) []byte {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 32 {
		return trimmed
	}
	decoded, err := base64.StdEncoding.DecodeString(string(trimmed))
	if err == nil {
		return decoded
	}
	return raw
}

func UpdateSyncProfileRun(tx *gorm.DB, profile *model.XUISyncProfile, status string, summary any, at int64) error {
	if at == 0 {
		at = time.Now().Unix()
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return tx.Model(profile).Updates(map[string]any{
		"last_run_at":      at,
		"last_run_status":  status,
		"last_run_summary": raw,
		"updated_at":       at,
	}).Error
}
