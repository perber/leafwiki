package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// Message is a single outgoing email.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Client sends email over SMTP using stdlib net/smtp. Implicit TLS (port 465,
// SecurityTLS) has no built-in stdlib helper, so it dials tls.Dial manually
// before handing the connection to smtp.NewClient; SecurityStartTLS and
// SecurityNone both dial plain TCP first and, for StartTLS, upgrade
// afterward. Pure Go / no cgo, so this is unaffected by cross-compilation
// for Windows or ARM (Raspberry Pi).
type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// Send delivers msg, honoring ctx's deadline for the whole SMTP round-trip
// (connect, optional STARTTLS, optional auth, and DATA). Callers are
// expected to derive ctx with Config.TimeoutOrDefault (see Service.send) —
// Client itself does not apply a default.
func (c *Client) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)

	conn, err := c.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer func() { _ = client.Close() }()

	if c.cfg.Security == SecurityStartTLS {
		tlsConfig := &tls.Config{ServerName: c.cfg.Host, InsecureSkipVerify: c.cfg.InsecureSkipVerify} //nolint:gosec // InsecureSkipVerify is an explicit, off-by-default operator opt-in
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if c.cfg.Username != "" {
		auth := smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(c.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(c.buildRawMessage(msg))); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}

	return client.Quit()
}

func (c *Client) dial(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{}
	if c.cfg.Security == SecurityTLS {
		tlsConfig := &tls.Config{ServerName: c.cfg.Host, InsecureSkipVerify: c.cfg.InsecureSkipVerify} //nolint:gosec // see Send's identical comment
		return tls.DialWithDialer(&d, "tcp", addr, tlsConfig)
	}
	return d.DialContext(ctx, "tcp", addr)
}

const mimeBoundary = "leafwiki-boundary-9f3a2c1e"

// buildRawMessage renders msg (plus From/FromName from cfg) as an RFC 5322
// message. Header values are stripped of CR/LF before use — msg.To is a
// stored user email address and, in principle, an admin-invite recipient
// address is attacker-influenced input, so this closes off header injection
// even though nothing upstream is known to allow embedding CRLF in an email
// field today (defense in depth, not a fix for a known bypass).
func (c *Client) buildRawMessage(msg Message) string {
	from := sanitizeHeader(c.cfg.From)
	if c.cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", sanitizeHeader(c.cfg.FromName), from)
	}

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + sanitizeHeader(msg.To) + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(msg.Subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTML != "" {
		b.WriteString("Content-Type: multipart/alternative; boundary=" + mimeBoundary + "\r\n\r\n")
		b.WriteString("--" + mimeBoundary + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.Text + "\r\n\r\n")
		b.WriteString("--" + mimeBoundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.HTML + "\r\n\r\n")
		b.WriteString("--" + mimeBoundary + "--\r\n")
	} else {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.Text + "\r\n")
	}

	return b.String()
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}
