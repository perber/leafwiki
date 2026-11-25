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
func warnOnUntrustedForwarding(logger *slog.Logger, engine *gin.Engine) gin.HandlerFunc {
	var once sync.Once
	return func(c *gin.Context) {
		if hasForwardingHeader(c.Request) && !peerIsTrusted(c, engine) {
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

// peerIsTrusted reports whether gin itself believed the forwarding header on
// this request, reading that decision back instead of reimplementing gin's
// CIDR matching (the previous version of this function did, and could
// silently drift from gin's actual behavior on a library upgrade).
//
// For a non-unix peer, gin's own (*Context).ClientIP applies the header only
// when it trusted the peer and the header parsed: it returns the header's
// value in that case, and RemoteIP() unchanged otherwise. Comparing the two
// — both already computed by gin, not by us — reads that decision back
// exactly, for whatever CIDR/IP-matching rules gin currently implements.
//
// A unix-socket peer is a special case gin's ClientIP doesn't expose this
// way: gin always treats it as locally trusted, so ClientIP alone can't tell
// "trusted, no header to apply" apart from "trusted, header applied but
// coincidentally identical." configureClientAddress is the one place that
// already decides whether the header is honored at all on a unix listener
// (ForwardedByClientIP, gated on whether any proxy was configured) — read
// that resolved field back directly rather than recomputing it.
func peerIsTrusted(c *gin.Context, engine *gin.Engine) bool {
	if addr, ok := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && strings.HasPrefix(addr.Network(), "unix") {
		return engine.ForwardedByClientIP
	}
	return c.ClientIP() != c.RemoteIP()
}
