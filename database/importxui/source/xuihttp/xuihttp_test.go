package xuihttp

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSourceAcquireSynthesizesSQLite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/login":
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/panel/api/inbounds/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"obj":[{"id":1,"remark":"demo","enable":true,"port":443,"protocol":"vless","settings":"{\"clients\":[{\"email\":\"a@example.com\",\"id\":\"11111111-1111-1111-1111-111111111111\"}]}","streamSettings":"{}","sniffing":"{}"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	path, cleanup, err := New(server.URL, "admin", "secret").Acquire(context.Background())
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM client_traffics").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one synthesized client_traffic row, got %d", count)
	}
}
