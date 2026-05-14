package common

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const passwordHashPrefix = "bcrypt:"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return passwordHashPrefix + string(hash), nil
}

func IsPasswordHash(password string) bool {
	return strings.HasPrefix(password, passwordHashPrefix) || strings.HasPrefix(password, "$2a$") ||
		strings.HasPrefix(password, "$2b$") || strings.HasPrefix(password, "$2y$")
}

func CheckPassword(storedPassword string, password string) (bool, bool) {
	if strings.HasPrefix(storedPassword, passwordHashPrefix) {
		hash := strings.TrimPrefix(storedPassword, passwordHashPrefix)
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil, false
	}
	if IsPasswordHash(storedPassword) {
		return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) == nil, true
	}
	return storedPassword == password, true
}
