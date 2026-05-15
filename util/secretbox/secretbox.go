package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"

	"github.com/deposist/s-ui-rus-inst/util/common"
	"golang.org/x/crypto/hkdf"
)

const Prefix = "sbox:v1:"

var (
	salt = []byte("s-ui secretbox v1")
	info = []byte("settings secrets")
)

type Box struct {
	aead cipher.AEAD
}

func New(masterKey []byte) (*Box, error) {
	if len(masterKey) == 0 {
		return nil, common.NewError("empty secretbox key")
	}
	key := make([]byte, 32)
	reader := hkdf.New(sha256.New, masterKey, salt, info)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func NewFromString(masterKey string) (*Box, error) {
	masterKey = strings.TrimSpace(masterKey)
	if decoded, err := base64.StdEncoding.DecodeString(masterKey); err == nil && len(decoded) > 0 {
		return New(decoded)
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(masterKey); err == nil && len(decoded) > 0 {
		return New(decoded)
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(masterKey); err == nil && len(decoded) > 0 {
		return New(decoded)
	}
	return New([]byte(masterKey))
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

func (b *Box) EncryptString(plaintext string, associatedData string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(plaintext), []byte(associatedData))
	payload := append(nonce, ciphertext...)
	return Prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (b *Box) DecryptString(value string, associatedData string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, Prefix))
	if err != nil {
		return "", err
	}
	nonceSize := b.aead.NonceSize()
	if len(payload) < nonceSize {
		return "", common.NewError("invalid secretbox payload")
	}
	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, []byte(associatedData))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
