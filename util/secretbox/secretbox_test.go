package secretbox

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	box, err := NewFromString("test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := box.EncryptString("telegram-token", "telegramBotToken")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "telegram-token" || !IsEncrypted(encrypted) {
		t.Fatalf("value was not encrypted: %q", encrypted)
	}
	decrypted, err := box.DecryptString(encrypted, "telegramBotToken")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "telegram-token" {
		t.Fatalf("unexpected plaintext %q", decrypted)
	}
}

func TestDecryptRejectsWrongAssociatedData(t *testing.T) {
	box, err := NewFromString("test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := box.EncryptString("telegram-token", "telegramBotToken")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := box.DecryptString(encrypted, "telegramProxyURL"); err == nil {
		t.Fatal("expected decrypt to fail with wrong associated data")
	}
}
