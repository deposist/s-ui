package util

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/deposist/s-ui-rus-inst/database/model"
)

func TestLinkGeneratorTUICIncludesUDPRelayMode(t *testing.T) {
	link := generateTUICLinkForTest(t, `{"udp_relay_mode":"native"}`)
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}

	if got := u.Query().Get("udp_relay_mode"); got != "native" {
		t.Fatalf("expected udp_relay_mode=native, got %q in %s", got, link)
	}
}

func TestLinkGeneratorTUICRoundTripPreservesUDPRelayMode(t *testing.T) {
	link := generateTUICLinkForTest(t, `{"udp_relay_mode":"quic"}`)
	outbound, _, err := GetOutbound(link, 0)
	if err != nil {
		t.Fatal(err)
	}

	if got := (*outbound)["udp_relay_mode"]; got != "quic" {
		t.Fatalf("expected round-trip udp_relay_mode=quic, got %#v", got)
	}
}

func TestLinkGeneratorTUICDefaultsUDPRelayMode(t *testing.T) {
	link := generateTUICLinkForTest(t, `{}`)
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}

	if got := u.Query().Get("udp_relay_mode"); got != defaultTUICUDPRelayMode {
		t.Fatalf("expected default udp_relay_mode=%s, got %q in %s", defaultTUICUDPRelayMode, got, link)
	}
}

func generateTUICLinkForTest(t *testing.T, outJSON string) string {
	t.Helper()

	clientConfig := json.RawMessage(`{
		"tuic": {
			"uuid": "11111111-1111-4111-8111-111111111111",
			"password": "secret"
		}
	}`)
	inbound := &model.Inbound{
		Type:    "tuic",
		Tag:     "tuic-test",
		Addrs:   json.RawMessage(`[]`),
		OutJson: json.RawMessage(outJSON),
		Options: json.RawMessage(`{
			"listen_port": 443,
			"congestion_control": "bbr"
		}`),
	}

	links := LinkGenerator(clientConfig, inbound, "example.com")
	if len(links) != 1 {
		t.Fatalf("expected one generated link, got %d: %#v", len(links), links)
	}
	return links[0]
}
