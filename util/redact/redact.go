package redact

import "strings"

const Marker = "[REDACTED]"

var sensitiveKeyFragments = []string{
	"authorization",
	"cookie",
	"password",
	"private",
	"secret",
	"token",
	"access_key",
	"client_secret",
	"subscription",
}

func Value(value any) any {
	switch v := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(v))
		for key, item := range v {
			if IsSensitiveKey(key) {
				redacted[key] = Marker
				continue
			}
			redacted[key] = Value(item)
		}
		return redacted
	case []any:
		redacted := make([]any, len(v))
		for i, item := range v {
			redacted[i] = Value(item)
		}
		return redacted
	default:
		return value
	}
}

func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
