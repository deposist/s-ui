package network

import (
	"net"

	"github.com/deposist/s-ui-rus-inst/logger"
)

// ListenWithFallback opens a TCP listener on `addr`. When `host` is a literal
// IP that the host kernel cannot bind (the typical "EADDRNOTAVAIL" error
// after restoring a backup from another machine), the function logs a
// warning and retries on every interface (host = ""). This keeps the panel
// reachable so the operator can correct the listen address from the UI.
//
// `host` is the bare host portion (no port) used by the caller to build
// `addr`; pass an empty string when the address is already an "any"
// address or when no fallback is desired.
func ListenWithFallback(addr, host string, port string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}
	if !shouldFallback(err) || host == "" {
		return nil, err
	}
	fallback := net.JoinHostPort("", port)
	logger.Warningf(
		"could not bind on %s (%v); falling back to %s. Update the listen address from the UI to silence this warning.",
		addr, err, fallback,
	)
	return net.Listen("tcp", fallback)
}

// shouldFallback reports whether err is the kind of bind error that points
// at a stale listen address inherited from another machine (the address is
// syntactically valid but the kernel does not own it).
func shouldFallback(err error) bool {
	return isAddrNotAvailable(err)
}
