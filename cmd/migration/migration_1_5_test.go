package migration

import "testing"

func TestTo15AddsClientIPSchemaAndBackfillsSubSecretsIdempotently(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec(`
CREATE TABLE clients (
	id integer PRIMARY KEY AUTOINCREMENT,
	enable boolean,
	name text
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO clients(enable, name) VALUES(1, 'alice'), (1, 'bob')").Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := to1_5(db); err != nil {
			t.Fatal(err)
		}
	}

	for _, column := range []string{
		"limit_ip",
		"ip_limit_mode",
		"last_online",
		"last_ip_count",
		"sub_secret",
	} {
		hasColumn, err := sqliteHasColumn(db, "clients", column)
		if err != nil {
			t.Fatal(err)
		}
		if !hasColumn {
			t.Fatalf("clients.%s was not added", column)
		}
	}
	if !db.Migrator().HasTable("client_ips") {
		t.Fatal("client_ips table was not created")
	}
	for _, column := range []string{"ip_hash", "ip_display"} {
		hasColumn, err := sqliteHasColumn(db, "client_ips", column)
		if err != nil {
			t.Fatal(err)
		}
		if !hasColumn {
			t.Fatalf("client_ips.%s was not added", column)
		}
	}
	if err := db.Exec(`
INSERT INTO client_ips(client_name, ip, first_seen, last_seen)
VALUES('alice', '198.51.100.10', 1, 1)
`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
INSERT INTO client_ips(client_name, ip, first_seen, last_seen)
VALUES('alice', '198.51.100.10', 1, 1)
`).Error; err == nil {
		t.Fatal("client_ips unique client/ip index was not created")
	}

	var clients []struct {
		Name      string
		SubSecret string
	}
	if err := db.Raw("SELECT name, sub_secret FROM clients ORDER BY name").Scan(&clients).Error; err != nil {
		t.Fatal(err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected two clients, got %d", len(clients))
	}
	if clients[0].SubSecret == "" || clients[1].SubSecret == "" || clients[0].SubSecret == clients[1].SubSecret {
		t.Fatalf("sub_secret backfill failed: %#v", clients)
	}
}

func TestTo15BackfillsClientIPHashesIdempotently(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.Exec(`
CREATE TABLE settings (
	id integer PRIMARY KEY AUTOINCREMENT,
	key text,
	value text
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO settings(key, value) VALUES('installSalt', 'test-salt')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
CREATE TABLE clients (
	id integer PRIMARY KEY AUTOINCREMENT,
	enable boolean,
	name text
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO clients(enable, name) VALUES(1, 'alice')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
CREATE TABLE client_ips (
	id integer PRIMARY KEY AUTOINCREMENT,
	client_name text,
	ip text,
	first_seen integer,
	last_seen integer
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
INSERT INTO client_ips(client_name, ip, first_seen, last_seen)
VALUES('alice', '198.51.100.10', 1, 1)
`).Error; err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := to1_5(db); err != nil {
			t.Fatal(err)
		}
	}

	var row struct {
		IP        string
		IPHash    string
		IPDisplay *string
	}
	if err := db.Raw("SELECT ip, ip_hash, ip_display FROM client_ips WHERE client_name = ?", "alice").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.IP != "198.51.100.10" {
		t.Fatalf("legacy ip column should remain additive, got %q", row.IP)
	}
	if row.IPHash == "" || row.IPHash == row.IP {
		t.Fatalf("ip_hash was not backfilled: %#v", row)
	}
	if row.IPDisplay != nil {
		t.Fatalf("ip_display should stay NULL by default: %#v", row.IPDisplay)
	}
}
