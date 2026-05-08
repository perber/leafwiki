package auth

import (
	"fmt"
	"net"
	"strings"
)

// TrustedProxies holds a set of trusted IP addresses and CIDR ranges.
// Only requests originating from these addresses will have Remote-User headers honoured.
type TrustedProxies struct {
	nets []*net.IPNet
	ips  []net.IP
}

// ParseTrustedProxies parses a comma-separated list of IP addresses and CIDR ranges.
// An empty string returns an empty (trust-nobody) list without error.
func ParseTrustedProxies(raw string) (*TrustedProxies, error) {
	tp := &TrustedProxies{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", entry, err)
			}
			tp.nets = append(tp.nets, ipNet)
		} else {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP address %q", entry)
			}
			tp.ips = append(tp.ips, ip)
		}
	}
	return tp, nil
}

// IsTrusted reports whether remoteAddr (host:port or bare host) is in the trusted set.
func (tp *TrustedProxies) IsTrusted(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, trusted := range tp.ips {
		if trusted.Equal(ip) {
			return true
		}
	}
	for _, ipNet := range tp.nets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
