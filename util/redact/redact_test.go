package redact

import "testing"

func TestValueRedactsSensitiveKeys(t *testing.T) {
	input := map[string]any{
		"user":  "admin",
		"token": "secret-token",
		"nested": map[string]any{
			"password": "secret-password",
			"port":     2095,
		},
	}
	redacted := Value(input).(map[string]any)
	if redacted["user"] != "admin" {
		t.Fatalf("non-secret field changed: %#v", redacted["user"])
	}
	if redacted["token"] != Marker {
		t.Fatalf("token was not redacted: %#v", redacted["token"])
	}
	nested := redacted["nested"].(map[string]any)
	if nested["password"] != Marker {
		t.Fatalf("password was not redacted: %#v", nested["password"])
	}
	if nested["port"] != 2095 {
		t.Fatalf("non-secret nested field changed: %#v", nested["port"])
	}
}
