package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestWriteUsage_UsesLongFlags(t *testing.T) {
	var buf bytes.Buffer

	writeUsage(&buf)

	output := buf.String()
	for _, expected := range []string{
		"--jwt-secret",
		"--admin-password",
		"--allow-insecure",
		"--data-dir",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected usage output to contain %q, got %q", expected, output)
		}
	}
}

func TestRegisterFlags_AcceptsSingleDashLongFlags(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{
		"-jwt-secret=test-secret",
		"-admin-password=test-password",
		"-allow-insecure=true",
	})
	if err != nil {
		t.Fatalf("expected single-dash long flags to parse, got %v (%s)", err, errOut.String())
	}

	if got := *flags.jwtSecret; got != "test-secret" {
		t.Fatalf("expected jwt secret %q, got %q", "test-secret", got)
	}
	if got := *flags.adminPassword; got != "test-password" {
		t.Fatalf("expected admin password %q, got %q", "test-password", got)
	}
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
	}
}

func TestValidateHTTPRemoteUserConfig(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		trustedProxyIPs string
		wantErr         bool
	}{
		{"disabled, no IPs", false, "", false},
		{"disabled, with IPs", false, "127.0.0.1", false},
		{"enabled, with IPs", true, "127.0.0.1", false},
		{"enabled, multiple IPs", true, "127.0.0.1,172.18.0.0/16", false},
		{"enabled, no IPs", true, "", true},
		{"enabled, whitespace only", true, "   ", true},
		{"enabled, commas only", true, ",,,", true},
		{"enabled, commas and whitespace", true, " , , ", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHTTPRemoteUserConfig(tc.enabled, tc.trustedProxyIPs)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateHTTPRemoteUserConfig(%v, %q) error = %v, wantErr %v", tc.enabled, tc.trustedProxyIPs, err, tc.wantErr)
			}
		})
	}
}

func TestRegisterFlags_AcceptsDoubleDashLongFlags(t *testing.T) {
	fs := flag.NewFlagSet("leafwiki", flag.ContinueOnError)
	var errOut bytes.Buffer
	fs.SetOutput(&errOut)
	flags := registerFlags(fs)

	err := fs.Parse([]string{
		"--jwt-secret=test-secret",
		"--admin-password=test-password",
		"--allow-insecure=true",
	})
	if err != nil {
		t.Fatalf("expected double-dash long flags to parse, got %v (%s)", err, errOut.String())
	}

	if got := *flags.jwtSecret; got != "test-secret" {
		t.Fatalf("expected jwt secret %q, got %q", "test-secret", got)
	}
	if got := *flags.adminPassword; got != "test-password" {
		t.Fatalf("expected admin password %q, got %q", "test-password", got)
	}
	if !*flags.allowInsecure {
		t.Fatalf("expected allow-insecure to be true")
	}
}
