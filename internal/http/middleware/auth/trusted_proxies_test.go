package auth_test

import (
	"testing"

	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

func TestParseTrustedProxies_Empty(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp.IsTrusted("127.0.0.1") {
		t.Error("empty list should trust nobody")
	}
}

func TestParseTrustedProxies_SingleIP(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tp.IsTrusted("127.0.0.1") {
		t.Error("127.0.0.1 should be trusted")
	}
	if tp.IsTrusted("192.168.1.1") {
		t.Error("192.168.1.1 should not be trusted")
	}
}

func TestParseTrustedProxies_MultipleIPs(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("127.0.0.1, ::1, 10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, addr := range []string{"127.0.0.1", "::1", "10.0.0.1"} {
		if !tp.IsTrusted(addr) {
			t.Errorf("%s should be trusted", addr)
		}
	}
	if tp.IsTrusted("10.0.0.2") {
		t.Error("10.0.0.2 should not be trusted")
	}
}

func TestParseTrustedProxies_CIDR(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("172.18.0.0/16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tp.IsTrusted("172.18.0.1") {
		t.Error("172.18.0.1 should be trusted (within CIDR)")
	}
	if !tp.IsTrusted("172.18.255.254") {
		t.Error("172.18.255.254 should be trusted (within CIDR)")
	}
	if tp.IsTrusted("172.19.0.1") {
		t.Error("172.19.0.1 should not be trusted (outside CIDR)")
	}
}

func TestParseTrustedProxies_Mixed(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("127.0.0.1, ::1, 172.18.0.0/16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tp.IsTrusted("127.0.0.1") {
		t.Error("127.0.0.1 should be trusted")
	}
	if !tp.IsTrusted("::1") {
		t.Error("::1 should be trusted")
	}
	if !tp.IsTrusted("172.18.42.10") {
		t.Error("172.18.42.10 should be trusted (CIDR)")
	}
}

func TestParseTrustedProxies_InvalidIP(t *testing.T) {
	_, err := authmw.ParseTrustedProxies("not-an-ip")
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestParseTrustedProxies_InvalidCIDR(t *testing.T) {
	_, err := authmw.ParseTrustedProxies("999.0.0.0/8")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestIsTrusted_WithPort(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// remoteAddr typically comes as "host:port" from net/http
	if !tp.IsTrusted("127.0.0.1:54321") {
		t.Error("127.0.0.1:54321 should be trusted (port stripped)")
	}
}

func TestIsTrusted_InvalidAddr(t *testing.T) {
	tp, err := authmw.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp.IsTrusted("not-valid") {
		t.Error("invalid address should not be trusted")
	}
}
