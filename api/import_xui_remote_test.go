package api

import (
	"testing"

	"github.com/deposist/s-ui-rus-inst/database/importxui"
)

func TestSyncSafeHostPortUsesURLParser(t *testing.T) {
	tests := map[string]struct {
		source importxui.SyncProfileSource
		want   string
	}{
		"source host and port": {
			source: importxui.SyncProfileSource{Host: "2001:db8::1", Port: 22},
			want:   "[2001:db8::1]:22",
		},
		"ssh url with userinfo": {
			source: importxui.SyncProfileSource{URL: "ssh://admin:secret@example.com:2222/etc/x-ui/x-ui.db"},
			want:   "example.com:2222",
		},
		"https url with ipv6": {
			source: importxui.SyncProfileSource{URL: "https://[2001:db8::2]:8443/panel"},
			want:   "[2001:db8::2]:8443",
		},
		"invalid url": {
			source: importxui.SyncProfileSource{URL: "://missing-scheme"},
			want:   "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := syncSafeHostPort(tc.source); got != tc.want {
				t.Fatalf("syncSafeHostPort() = %q, want %q", got, tc.want)
			}
		})
	}
}
