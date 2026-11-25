package oidc

import "testing"

func TestConfig_Enabled(t *testing.T) {
	if (Config{}).Enabled() {
		t.Fatal("zero-value Config must be disabled")
	}
	if !(Config{Issuer: "https://auth.example.com/"}).Enabled() {
		t.Fatal("a Config with an Issuer must be enabled")
	}
}
