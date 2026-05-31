package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultEndpoint        = "http://127.0.0.1:8080/mcp"
	DefaultRequestTimeout  = 2 * time.Minute
	DefaultShutdownTimeout = 5 * time.Second
	DefaultMaxFrameSize    = 128 * 1024 * 1024
)

var errFrameTooLarge = errors.New("json-rpc frame exceeds max frame size")

type Config struct {
	Endpoint        string
	APIKey          string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	MaxFrameSize    int64
	HTTPClient      *http.Client
}

func DefaultConfig() Config {
	return Config{
		Endpoint:        DefaultEndpoint,
		RequestTimeout:  DefaultRequestTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
		MaxFrameSize:    DefaultMaxFrameSize,
	}
}

func (c Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("invalid endpoint: empty")
	}
	parsed, err := url.Parse(c.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid endpoint %q", c.Endpoint)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid endpoint %q: scheme must be http or https", c.Endpoint)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if c.MaxFrameSize <= 0 {
		return fmt.Errorf("max frame size must be positive")
	}
	return nil
}

func Run(ctx context.Context, cfg Config, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{}
	}

	p := &proxy{
		cfg:     cfg,
		client:  client,
		stdout:  stdout,
		stderr:  stderr,
		redact:  newRedactor(cfg.APIKey),
		outLock: &sync.Mutex{},
	}

	frames := make(chan frameRead, 1)
	go readFrames(stdin, cfg.MaxFrameSize, frames)

	for {
		select {
		case <-ctx.Done():
			p.processPendingFrames(ctx, frames, 25*time.Millisecond)
			p.shutdown()
			return nil
		case frame, ok := <-frames:
			if !ok {
				p.shutdown()
				return nil
			}
			p.handleFrame(ctx, frame)
		}
	}
}

type proxy struct {
	cfg             Config
	client          *http.Client
	stdout          io.Writer
	stderr          io.Writer
	redact          redactor
	outLock         *sync.Mutex
	sessionID       string
	protocolVersion string
}

type frameRead struct {
	body []byte
	err  error
}

type frameInfo struct {
	hasResponse bool
	id          json.RawMessage
	method      string
}

func (p *proxy) processPendingFrames(ctx context.Context, frames <-chan frameRead, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()

	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return
			}
			p.handleFrame(ctx, frame)
			return
		case <-timer.C:
			return
		}
	}
}

func (p *proxy) handleFrame(ctx context.Context, frame frameRead) {
	if errors.Is(frame.err, errFrameTooLarge) {
		p.writeJSONRPCError(nil, -32000, "JSON-RPC frame exceeds configured max frame size", nil)
		p.logf("rejected oversized JSON-RPC frame")
		return
	}
	if frame.err != nil {
		p.writeJSONRPCError(nil, -32000, "Failed to read JSON-RPC frame", nil)
		p.logf("failed to read JSON-RPC frame: %v", frame.err)
		return
	}

	info, err := parseFrameInfo(frame.body)
	if err != nil {
		p.writeJSONRPCError(nil, -32700, "Parse error", nil)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(frame.body))
	if err != nil {
		p.writeJSONRPCError(info.idOrNil(), -32000, "Upstream MCP request failed", nil)
		p.logf("failed to create upstream request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	if p.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", p.sessionID)
	}
	if p.protocolVersion != "" {
		req.Header.Set("Mcp-Protocol-Version", p.protocolVersion)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.handleTransportError(info, err)
		return
	}
	defer resp.Body.Close()

	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		p.sessionID = sessionID
	}

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.handleHTTPStatusError(info, resp.StatusCode, contentType)
		return
	}

	if isSSE(contentType) {
		p.handleContentError(info, "SSE is unsupported by leafwiki-mcp-stdio", contentType)
		return
	}
	if contentType != "" && !isJSONContentType(contentType) {
		p.handleContentError(info, "unsupported upstream content type", contentType)
		return
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		p.handleTransportError(info, readErr)
		return
	}

	if len(body) == 0 {
		if !info.hasResponse {
			return
		}
		p.writeJSONRPCError(info.idOrNil(), -32000, "Upstream MCP response was empty", nil)
		p.logf("upstream MCP response was empty")
		return
	}

	if !isJSONContentType(contentType) {
		p.handleContentError(info, "unsupported upstream content type", contentType)
		return
	}

	p.writeProtocolBody(body)
	if info.method == "initialize" {
		p.updateProtocolVersion(body)
	}
}

func (p *proxy) handleTransportError(info frameInfo, err error) {
	p.logf("upstream request failed: %v", err)
	if info.hasResponse {
		p.writeJSONRPCError(info.idOrNil(), -32000, "Upstream MCP request failed", nil)
	}
}

func (p *proxy) handleHTTPStatusError(info frameInfo, status int, contentType string) {
	msg := "Upstream MCP request failed"
	if status == http.StatusNotFound && p.sessionID != "" {
		msg = "Upstream MCP session is missing or expired"
	}
	p.logf("%s: HTTP %d", strings.ToLower(msg), status)
	if info.hasResponse {
		data := map[string]any{"status": status}
		if contentType != "" {
			data["contentType"] = contentType
		}
		p.writeJSONRPCError(info.idOrNil(), -32000, msg, data)
	}
}

func (p *proxy) handleContentError(info frameInfo, msg, contentType string) {
	p.logf("%s: %s", msg, contentType)
	if info.hasResponse {
		data := map[string]any{}
		if contentType != "" {
			data["contentType"] = contentType
		}
		p.writeJSONRPCError(info.idOrNil(), -32000, msg, data)
	}
}

func (p *proxy) writeProtocolBody(body []byte) {
	p.outLock.Lock()
	defer p.outLock.Unlock()
	_, _ = p.stdout.Write(body)
	_, _ = p.stdout.Write([]byte("\n"))
}

func (p *proxy) writeJSONRPCError(id json.RawMessage, code int, message string, data any) {
	errObj := map[string]any{
		"code":    code,
		"message": message,
	}
	if data != nil {
		errObj["data"] = data
	}
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      rawOrNil(id),
		"error":   errObj,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return
	}
	p.writeProtocolBody(body)
}

func rawOrNil(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func (p *proxy) updateProtocolVersion(body []byte) {
	var resp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && resp.Result.ProtocolVersion != "" {
		p.protocolVersion = resp.Result.ProtocolVersion
	}
}

func (p *proxy) shutdown() {
	if p.sessionID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.ShutdownTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.cfg.Endpoint, nil)
	if err != nil {
		p.logf("cleanup warning: failed to create upstream DELETE: %v", err)
		return
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", p.sessionID)
	if p.protocolVersion != "" {
		req.Header.Set("Mcp-Protocol-Version", p.protocolVersion)
	}
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.logf("cleanup warning: upstream DELETE failed: %v", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		p.logf("cleanup warning: upstream DELETE returned HTTP %d", resp.StatusCode)
	}
}

func (p *proxy) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(p.stderr, p.redact.String(msg))
}

func readFrames(r io.Reader, maxFrameSize int64, out chan<- frameRead) {
	defer close(out)
	reader := bufio.NewReader(r)
	for {
		body, err := readLineLimited(reader, maxFrameSize)
		if len(body) > 0 {
			out <- frameRead{body: body}
		}
		if errors.Is(err, errFrameTooLarge) {
			out <- frameRead{err: err}
			continue
		}
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			out <- frameRead{err: err}
			return
		}
	}
}

func readLineLimited(reader *bufio.Reader, maxFrameSize int64) ([]byte, error) {
	var buf []byte
	for {
		part, err := reader.ReadSlice('\n')
		buf = append(buf, part...)
		if int64(len(buf)) > maxFrameSize {
			discardLine(reader, err)
			return nil, errFrameTooLarge
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(buf) == 0 {
				return nil, io.EOF
			}
			return bytes.TrimRight(buf, "\r\n"), nil
		}
		if err != nil {
			return nil, err
		}
		return bytes.TrimRight(buf, "\r\n"), nil
	}
}

func discardLine(reader *bufio.Reader, prior error) {
	if prior == nil || errors.Is(prior, io.EOF) {
		return
	}
	for errors.Is(prior, bufio.ErrBufferFull) {
		_, prior = reader.ReadSlice('\n')
	}
}

func parseFrameInfo(body []byte) (frameInfo, error) {
	if !json.Valid(body) {
		return frameInfo{}, errors.New("invalid json")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return frameInfo{}, errors.New("empty frame")
	}
	if trimmed[0] == '[' {
		return frameInfo{hasResponse: true}, nil
	}
	if trimmed[0] != '{' {
		return frameInfo{}, errors.New("frame must be object or batch")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return frameInfo{}, err
	}

	info := frameInfo{}
	if id, ok := obj["id"]; ok {
		info.id = append(json.RawMessage(nil), id...)
	}
	if method, ok := obj["method"]; ok {
		if err := json.Unmarshal(method, &info.method); err == nil && info.method != "" {
			_, info.hasResponse = obj["id"]
		}
	}
	if len(info.id) > 0 && info.method == "" && obj["result"] == nil && obj["error"] == nil {
		info.hasResponse = true
	}
	return info, nil
}

func (i frameInfo) idOrNil() json.RawMessage {
	if len(i.id) == 0 {
		return nil
	}
	return i.id
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

func isSSE(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.HasPrefix(strings.ToLower(contentType), "text/event-stream")
	}
	return mediaType == "text/event-stream"
}

type redactor struct {
	secrets []string
}

func newRedactor(secrets ...string) redactor {
	out := redactor{}
	for _, secret := range secrets {
		if secret != "" {
			out.secrets = append(out.secrets, secret)
		}
	}
	return out
}

func (r redactor) String(input string) string {
	out := input
	for _, secret := range r.secrets {
		out = strings.ReplaceAll(out, secret, "[REDACTED]")
	}
	return out
}
