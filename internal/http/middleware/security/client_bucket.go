package security

import (
	"net/netip"

	"github.com/gin-gonic/gin"
)

// unknownClientBucket is the one shared bucket for callers whose address
// cannot be attributed. Sharing a single budget fails closed: a request
// without a usable address never gets a bucket of its own.
const unknownClientBucket = "unknown"

// ipv6BucketBits is the prefix length that IPv6 addresses are grouped by. A
// /64 is the unit a network hands to one subscriber, so grouping by it stops a
// caller from switching to a fresh address for every request.
const ipv6BucketBits = 64

// ClientBucket names the group of addresses a request's client belongs to, for
// callers that keep a per-client budget. It starts from c.ClientIP(): IPv4
// stays as is, an IPv4-mapped IPv6 address becomes its IPv4 form, and any
// other IPv6 address is masked to its /64 and returned in canonical text form.
// An empty, unparsable or zoned address, and the literal "<nil>" that gin
// returns for a unix-socket peer without a forwarding header, all map to the
// single bucket "unknown".
func ClientBucket(c *gin.Context) string {
	addr, err := netip.ParseAddr(c.ClientIP())
	if err != nil || addr.Zone() != "" {
		return unknownClientBucket
	}

	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}

	prefix, err := addr.Prefix(ipv6BucketBits)
	if err != nil {
		return unknownClientBucket
	}
	return prefix.Addr().String()
}
