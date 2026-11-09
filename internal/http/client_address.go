package http

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
)

// configureClientAddress makes c.ClientIP() believe a forwarding header only
// from the proxies in trusted. Gin trusts every peer by default, which lets a
// direct client choose the address that logs and per-client limits see.
//
// Only X-Forwarded-For is read. Gin's default also reads X-Real-IP and takes
// the first header that parses, so a second header would give a client one
// more way to pick its own address. A nil or empty list trusts nobody.
//
// A unix-socket peer has no IP the list could name, and gin trusts it
// whatever the list says. Anyone who may open the socket could then choose
// their own address with the header, so on a unix socket the header is read
// only when the operator has listed at least one proxy, which is how they say
// that a proxy in front of the socket sets it.
func configureClientAddress(engine *gin.Engine, trusted *auth_middleware.TrustedProxies, unixListener bool) error {
	entries := trusted.Entries()
	if err := engine.SetTrustedProxies(entries); err != nil {
		return err
	}
	engine.RemoteIPHeaders = []string{"X-Forwarded-For"}
	engine.ForwardedByClientIP = !unixListener || len(entries) > 0
	return nil
}

// warnOnUntrustedForwarding logs one warning, per call of this function, when
// a forwarding header arrives from a peer that is not trusted. The header is
// ignored in that case, and the common cause is a reverse proxy missing from
// --trusted-proxy-ips: every visitor then shares the proxy's address. A direct
// client can send the header as well, so the wording does not assume a proxy.
func warnOnUntrustedForwarding(logger *slog.Logger, trusted *auth_middleware.TrustedProxies) gin.HandlerFunc {
	var once sync.Once
	return func(c *gin.Context) {
		if hasForwardingHeader(c.Request) && !peerIsTrusted(c.Request, trusted) {
			once.Do(func() {
				logger.Warn("forwarding header from an untrusted peer ignored; "+
					"if this is your reverse proxy, list it in --trusted-proxy-ips",
					"peer", c.RemoteIP())
			})
		}
		c.Next()
	}
}

func hasForwardingHeader(r *http.Request) bool {
	return r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != ""
}

// peerIsTrusted mirrors what configureClientAddress makes gin do, to decide
// whether a forwarding header is believed: a unix-socket peer is believed when
// at least one proxy is listed, any other peer when it is in the list. It is a
// mirror because gin does not expose its decision. InjectRemoteUser asks a
// different question (is the immediate TCP peer a proxy) and keeps using
// IsTrusted.
func peerIsTrusted(r *http.Request, trusted *auth_middleware.TrustedProxies) bool {
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && strings.HasPrefix(addr.Network(), "unix") {
		return len(trusted.Entries()) > 0
	}
	return trusted != nil && trusted.IsTrusted(r.RemoteAddr)
}
