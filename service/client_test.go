package service

import (
	"reflect"
	"testing"
)

func TestDecodeClientInbounds(t *testing.T) {
	got, ok := decodeClientInbounds(7, []byte(`[1,2,3]`), "test")
	if !ok {
		t.Fatal("valid inbounds should decode")
	}
	if want := []uint{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbounds: %#v, want %#v", got, want)
	}

	if _, ok := decodeClientInbounds(7, []byte(`{`), "test"); ok {
		t.Fatal("invalid inbounds should be rejected")
	}
}

func TestDecodeClientLinks(t *testing.T) {
	got, ok := decodeClientLinks(7, []byte(`[{"remark":"in","type":"local","uri":"vless://example"}]`), "test")
	if !ok {
		t.Fatal("valid links should decode")
	}
	want := []map[string]string{{
		"remark": "in",
		"type":   "local",
		"uri":    "vless://example",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected links: %#v, want %#v", got, want)
	}

	if _, ok := decodeClientLinks(7, []byte(`{`), "test"); ok {
		t.Fatal("invalid links should be rejected")
	}
}
