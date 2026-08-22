package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal plaintext SMTP server that accepts exactly one
// connection, speaks just enough of the protocol for Client.Send's SecurityNone
// path, and records the transcript so tests can assert on it. It exists so
// Client's protocol handling (EHLO/MAIL/RCPT/DATA/QUIT and message framing)
// is verified without requiring Docker/Mailpit — the plan's Mailpit-based
// integration test is a separate, TEST_SMTP_HOST-gated layer on top of this.
type fakeSMTPServer struct {
	addr string

	mailFrom string
	rcptTo   string
	data     string
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake smtp listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	srv := &fakeSMTPServer{addr: ln.Addr().String()}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		srv.serve(conn)
	}()

	return srv
}

func (s *fakeSMTPServer) serve(conn net.Conn) {
	reader := bufio.NewReader(conn)
	writeLine(conn, "220 fake.smtp.test ESMTP")

	inData := false
	var dataLines []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.data = strings.Join(dataLines, "\r\n")
				writeLine(conn, "250 OK: message accepted")
				continue
			}
			dataLines = append(dataLines, line)
			continue
		}

		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			writeLine(conn, "250-fake.smtp.test greets you")
			writeLine(conn, "250 8BITMIME")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			s.mailFrom = line
			writeLine(conn, "250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			s.rcptTo = line
			writeLine(conn, "250 OK")
		case upper == "DATA":
			inData = true
			writeLine(conn, "354 Start mail input; end with <CRLF>.<CRLF>")
		case upper == "QUIT":
			writeLine(conn, "221 Bye")
			return
		default:
			writeLine(conn, "500 unrecognized command")
		}
	}
}

func writeLine(conn net.Conn, line string) {
	_, _ = fmt.Fprintf(conn, "%s\r\n", line)
}

func TestClient_Send_PlaintextRoundTrip_DeliversMessage(t *testing.T) {
	srv := startFakeSMTPServer(t)
	host, port := splitHostPort(t, srv.addr)

	client := NewClient(Config{
		Host:     host,
		Port:     port,
		From:     "wiki@example.com",
		FromName: "LeafWiki",
		Security: SecurityNone,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Send(ctx, Message{
		To:      "user@example.com",
		Subject: "Reset your LeafWiki password",
		HTML:    "<p>link</p>",
		Text:    "link",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// The server sees a short settling window between accepting DATA and this
	// assertion running; give it a moment since the send goroutine and the
	// server goroutine are only loosely synchronized by the protocol itself.
	time.Sleep(50 * time.Millisecond)

	if !strings.Contains(srv.mailFrom, "wiki@example.com") {
		t.Errorf("expected MAIL FROM to contain sender, got %q", srv.mailFrom)
	}
	if !strings.Contains(srv.rcptTo, "user@example.com") {
		t.Errorf("expected RCPT TO to contain recipient, got %q", srv.rcptTo)
	}
	if !strings.Contains(srv.data, "Subject: Reset your LeafWiki password") {
		t.Errorf("expected message data to contain subject header, got: %s", srv.data)
	}
	if !strings.Contains(srv.data, "From: LeafWiki <wiki@example.com>") {
		t.Errorf("expected message data to contain From header with display name, got: %s", srv.data)
	}
}

func TestClient_BuildRawMessage_StripsCRLFFromHeaderValues(t *testing.T) {
	client := NewClient(Config{From: "wiki@example.com", FromName: "LeafWiki"})

	raw := client.buildRawMessage(Message{
		To:      "attacker@example.com\r\nBcc: victim@example.com",
		Subject: "hello\r\nX-Injected: true",
		Text:    "body",
	})

	if strings.Contains(raw, "\r\nBcc: victim@example.com") {
		t.Errorf("expected injected Bcc to not become its own header line, got: %s", raw)
	}
	if strings.Contains(raw, "\r\nX-Injected: true") {
		t.Errorf("expected injected header to not become its own header line, got: %s", raw)
	}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split addr %q: %v", addr, err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port %q: %v", portStr, err)
	}
	return host, port
}
