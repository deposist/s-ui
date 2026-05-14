package common

import "testing"

func TestPasswordHashAndMigrationChecks(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "secret" || !IsPasswordHash(hash) {
		t.Fatalf("password was not hashed with expected marker: %q", hash)
	}
	if ok, migrate := CheckPassword(hash, "secret"); !ok || migrate {
		t.Fatalf("hashed password check = %v, migrate = %v", ok, migrate)
	}
	if ok, migrate := CheckPassword("secret", "secret"); !ok || !migrate {
		t.Fatalf("plain password check = %v, migrate = %v", ok, migrate)
	}
	if ok, _ := CheckPassword(hash, "wrong"); ok {
		t.Fatal("wrong password was accepted")
	}
}
