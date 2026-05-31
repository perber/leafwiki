package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/perber/wiki/internal/wiki/mcpstdio"
)

type getenvFunc func(string) string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runCLI(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}

func runCLI(ctx context.Context, args []string, getenv getenvFunc, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg := mcpstdio.DefaultConfig()

	fs := flag.NewFlagSet("leafwiki-mcp-stdio", flag.ContinueOnError)
	fs.SetOutput(stderr)
	endpoint := fs.String("endpoint", "", "upstream LeafWiki MCP URL")
	apiKey := fs.String("api-key", "", "optional LeafWiki MCP API key sent as bearer auth")
	requestTimeout := fs.String("request-timeout", "", "per upstream HTTP request timeout")
	shutdownTimeout := fs.String("shutdown-timeout", "", "upstream shutdown DELETE timeout")
	maxFrameSize := fs.String("max-frame-size", "", "maximum single STDIO JSON-RPC frame size")
	help := fs.Bool("help", false, "show usage")
	fs.BoolVar(help, "h", false, "show usage")
	fs.Usage = func() {
		writeUsage(stderr)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			writeUsage(stdout)
			return 0
		}
		return 2
	}
	if *help {
		writeUsage(stdout)
		return 0
	}

	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	cfg.Endpoint = resolveString("endpoint", *endpoint, visited, getenv, "LEAFWIKI_MCP_ENDPOINT", cfg.Endpoint)
	cfg.APIKey = resolveString("api-key", *apiKey, visited, getenv, "LEAFWIKI_MCP_API_KEY", cfg.APIKey)

	var err error
	cfg.RequestTimeout, err = resolveDuration("request-timeout", *requestTimeout, visited, getenv, "LEAFWIKI_MCP_STDIO_REQUEST_TIMEOUT", cfg.RequestTimeout)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	cfg.ShutdownTimeout, err = resolveDuration("shutdown-timeout", *shutdownTimeout, visited, getenv, "LEAFWIKI_MCP_STDIO_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	cfg.MaxFrameSize, err = resolveBytes("max-frame-size", *maxFrameSize, visited, getenv, "LEAFWIKI_MCP_STDIO_MAX_FRAME_SIZE", cfg.MaxFrameSize)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := mcpstdio.Run(ctx, cfg, stdin, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func writeUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `leafwiki-mcp-stdio bridges MCP STDIO clients to LeafWiki's Streamable HTTP /mcp endpoint.

Usage:
  leafwiki-mcp-stdio [options]

Options:
  --endpoint          Upstream LeafWiki MCP URL (default: http://127.0.0.1:8080/mcp)
  --api-key           Optional lwk_... key sent as Authorization: Bearer
  --request-timeout   Per upstream HTTP request timeout (default: 2m)
  --shutdown-timeout  Time allowed for upstream DELETE cleanup (default: 5s)
  --max-frame-size    Maximum single STDIO JSON-RPC frame (default: 128MiB)
  --help              Show usage

Environment:
  LEAFWIKI_MCP_ENDPOINT
  LEAFWIKI_MCP_API_KEY
  LEAFWIKI_MCP_STDIO_REQUEST_TIMEOUT
  LEAFWIKI_MCP_STDIO_SHUTDOWN_TIMEOUT
  LEAFWIKI_MCP_STDIO_MAX_FRAME_SIZE
`)
}

func resolveString(name, flagValue string, visited map[string]bool, getenv getenvFunc, envName, fallback string) string {
	if visited[name] {
		return flagValue
	}
	if value := getenv(envName); value != "" {
		return value
	}
	return fallback
}

func resolveDuration(name, flagValue string, visited map[string]bool, getenv getenvFunc, envName string, fallback time.Duration) (time.Duration, error) {
	raw := ""
	if visited[name] {
		raw = flagValue
	} else {
		raw = getenv(envName)
	}
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	return value, nil
}

func resolveBytes(name, flagValue string, visited map[string]bool, getenv getenvFunc, envName string, fallback int64) (int64, error) {
	raw := ""
	if visited[name] {
		raw = flagValue
	} else {
		raw = getenv(envName)
	}
	if raw == "" {
		return fallback, nil
	}
	value, err := humanize.ParseBytes(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	return int64(value), nil
}
