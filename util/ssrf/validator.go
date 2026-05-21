package ssrf

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/util/common"
)

const lookupTimeout = 5 * time.Second

var (
	defaultAllowedSchemes = []string{"http", "https", "socks5"}
	blockedPrefixes       = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("::/128"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("ff00::/8"),
	}
)

func IsSafeOutboundURL(rawURL string) bool {
	return ValidateOutboundURL(context.Background(), rawURL) == nil
}

func ValidateOutboundURL(ctx context.Context, rawURL string, allowedSchemes ...string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if !isAllowedScheme(scheme, allowedSchemes) {
		return common.NewError("unsupported url scheme")
	}
	host := parsed.Hostname()
	if host == "" {
		return common.NewError("missing url host")
	}
	if strings.EqualFold(host, "localhost") {
		return common.NewError("localhost url is not allowed")
	}
	if port := parsed.Port(); port != "" {
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 || num > 65535 {
			return common.NewError("invalid url port")
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlockedAddr(addr) {
			return common.NewError("url host is not allowed")
		}
		return nil
	}
	if err := validateHostname(host); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolveCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return common.NewError("url host did not resolve")
	}
	for _, ipAddr := range addrs {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if ok {
			addr = addr.Unmap()
		}
		if !ok || isBlockedAddr(addr) {
			return common.NewError("url host resolves to a disallowed IP")
		}
	}
	return nil
}

func isAllowedScheme(scheme string, allowed []string) bool {
	if len(allowed) == 0 {
		allowed = defaultAllowedSchemes
	}
	for _, candidate := range allowed {
		if scheme == strings.ToLower(candidate) {
			return true
		}
	}
	return false
}

func validateHostname(host string) error {
	host = strings.TrimSuffix(host, ".")
	if len(host) == 0 || len(host) > 253 {
		return common.NewError("invalid url host")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return common.NewError("url hostname must include a valid TLD")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return common.NewError("invalid url host label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return common.NewError("invalid url host label")
		}
		for _, r := range label {
			if r > 127 || !(r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return common.NewError("invalid url host label")
			}
		}
	}
	tld := labels[len(labels)-1]
	if len(tld) < 2 || len(tld) > 63 {
		return common.NewError("invalid url TLD")
	}
	for _, r := range tld {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return common.NewError("invalid url TLD")
		}
	}
	return nil
}

func isBlockedAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
