package email

import (
	"testing"
	"time"
)

func TestConfig_Enabled_HostEmpty_ReturnsFalse(t *testing.T) {
	cfg := Config{}
	if cfg.Enabled() {
		t.Fatal("expected Enabled() to be false for zero-value Config")
	}
}

func TestConfig_Enabled_HostSet_ReturnsTrue(t *testing.T) {
	cfg := Config{Host: "smtp.example.com"}
	if !cfg.Enabled() {
		t.Fatal("expected Enabled() to be true when Host is set")
	}
}

func TestConfig_TimeoutOrDefault_Unset_ReturnsDefault(t *testing.T) {
	cfg := Config{}
	if got := cfg.TimeoutOrDefault(); got != defaultTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultTimeout, got)
	}
}

func TestConfig_TimeoutOrDefault_Set_ReturnsConfigured(t *testing.T) {
	cfg := Config{Timeout: 30 * time.Second}
	if got := cfg.TimeoutOrDefault(); got != 30*time.Second {
		t.Fatalf("expected configured timeout, got %v", got)
	}
}
