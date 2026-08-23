package auth

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/email"
)

// fakeInviteSMTPServer is a minimal single-connection plaintext SMTP server
// that captures the full DATA body, so a test can pull the invite link's
// token back out of the email InviteUserUseCase actually sent — exercising
// ConfirmInviteUseCase against a real, freshly issued token rather than a
// hand-constructed one.
type fakeInviteSMTPServer struct {
	addr string
	data chan string
}

func startFakeInviteSMTPServer(t *testing.T) *fakeInviteSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake smtp listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	srv := &fakeInviteSMTPServer{addr: ln.Addr().String(), data: make(chan string, 4)}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn)
		}
	}()
	return srv
}

func (s *fakeInviteSMTPServer) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = fmt.Fprintf(conn, "%s\r\n", line) }
	write("220 fake.smtp.test ESMTP")

	inData := false
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.data <- strings.Join(lines, "\n")
				write("250 OK: message accepted")
			} else {
				lines = append(lines, line)
			}
			continue
		}

		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			write("250 fake.smtp.test greets you")
		case strings.HasPrefix(upper, "MAIL FROM:"), strings.HasPrefix(upper, "RCPT TO:"):
			write("250 OK")
		case upper == "DATA":
			inData = true
			write("354 Start mail input; end with <CRLF>.<CRLF>")
		case upper == "QUIT":
			write("221 Bye")
			return
		default:
			write("500 unrecognized command")
		}
	}
}

var inviteTokenRe = regexp.MustCompile(`token=([A-Za-z0-9._-]+)`)

func extractToken(t *testing.T, emailData string) string {
	t.Helper()
	m := inviteTokenRe.FindStringSubmatch(emailData)
	if m == nil {
		t.Fatalf("could not find a token in the sent email:\n%s", emailData)
	}
	return m[1]
}

// setupEmailUseCases builds a real EmailTokenService (temp-dir SQLite stores,
// matching this package's other test fixtures) pointed at an SMTP target at
// smtpAddr — which callers are free to leave unreachable, since none of the
// tests here need a send to actually succeed; RequestPasswordResetAsync's
// whole point is to never make the caller wait on it.
func setupEmailUseCases(t *testing.T, smtpAddr string) (*coreauth.EmailTokenService, *coreauth.UserService, *coreauth.UserResolver, *coreauth.AuthService) {
	t.Helper()
	storageDir := t.TempDir()

	userStore, err := coreauth.NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := userStore.Close(); err != nil {
			t.Errorf("Close user store: %v", err)
		}
	})
	users := coreauth.NewUserService(userStore)
	usersFn := func() *coreauth.UserService { return users }

	resolver, err := coreauth.NewUserResolver(usersFn)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}

	sessionStore, err := coreauth.NewSessionStore(storageDir)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour*7)
	authSvc := coreauth.NewAuthService(users, sessions, nil)

	tokenStore, err := coreauth.NewEmailTokenStore(storageDir)
	if err != nil {
		t.Fatalf("NewEmailTokenStore: %v", err)
	}
	t.Cleanup(func() {
		if err := tokenStore.Close(); err != nil {
			t.Errorf("Close token store: %v", err)
		}
	})

	host, portStr, err := net.SplitHostPort(smtpAddr)
	if err != nil {
		t.Fatalf("failed to split smtp addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse smtp port: %v", err)
	}
	mailer := email.NewService(email.Config{
		Host: host, Port: port, From: "wiki@example.com",
		Security: email.SecurityNone, Timeout: 5 * time.Second,
	})

	svc := coreauth.NewEmailTokenService(tokenStore, users, authSvc, mailer, "https://wiki.example.com")
	return svc, users, resolver, authSvc
}

// TestRequestPasswordResetUseCase_Execute_SameResponseForKnownAndUnknownIdentifier
// pins down the single most security-critical behavior in this feature: the
// HTTP-facing response for /auth/password/forgot must be indistinguishable
// whether or not the identifier resolves to a real user, so a caller cannot
// enumerate valid usernames/emails via this endpoint.
func TestRequestPasswordResetUseCase_Execute_SameResponseForKnownAndUnknownIdentifier(t *testing.T) {
	svc, users, _, _ := setupEmailUseCases(t, "127.0.0.1:1") // unreachable: dispatch is async, so this must not matter
	uc := NewRequestPasswordResetUseCase(func() *coreauth.EmailTokenService { return svc })

	if _, err := users.CreateUser("alice", "alice@example.com", "password123", coreauth.RoleEditor); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	errKnown := uc.Execute(context.Background(), RequestPasswordResetInput{Identifier: "alice"})
	errUnknown := uc.Execute(context.Background(), RequestPasswordResetInput{Identifier: "no-such-user"})

	if errKnown != nil {
		t.Fatalf("expected nil for a known identifier, got %v", errKnown)
	}
	if errUnknown != nil {
		t.Fatalf("expected nil for an unknown identifier, got %v", errUnknown)
	}
}

// TestRequestPasswordResetUseCase_Execute_DoesNotBlockOnSlowSMTP proves the
// enumeration-timing protection isn't just "returns the same value" but
// "returns without waiting on the network at all" — a handler whose latency
// depended on SMTP round-trip time would leak the same information via a
// timing side channel even while returning an identical body.
func TestRequestPasswordResetUseCase_Execute_DoesNotBlockOnSlowSMTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		close(accepted)
		// Deliberately never send the SMTP greeting or close the connection —
		// simulates a stalled SMTP server. Held open until the test process
		// tears down the listener.
		<-context.Background().Done()
		_ = conn
	}()

	svc, users, _, _ := setupEmailUseCases(t, ln.Addr().String())
	uc := NewRequestPasswordResetUseCase(func() *coreauth.EmailTokenService { return svc })

	if _, err := users.CreateUser("alice", "alice@example.com", "password123", coreauth.RoleEditor); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	start := time.Now()
	if err := uc.Execute(context.Background(), RequestPasswordResetInput{Identifier: "alice"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected Execute to return without waiting for the SMTP send, took %v", elapsed)
	}

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the background send to even connect — Execute may not be dispatching at all")
	}
}

func TestRequestPasswordResetUseCase_Execute_EmailDisabled_ReturnsErrEmailDisabled(t *testing.T) {
	uc := NewRequestPasswordResetUseCase(func() *coreauth.EmailTokenService { return nil })

	if err := uc.Execute(context.Background(), RequestPasswordResetInput{Identifier: "alice"}); err != coreauth.ErrEmailDisabled {
		t.Fatalf("expected ErrEmailDisabled, got %v", err)
	}
}

func TestConfirmPasswordResetUseCase_Execute_WeakPassword_ReturnsValidationError(t *testing.T) {
	svc, _, _, _ := setupEmailUseCases(t, "127.0.0.1:1")
	uc := NewConfirmPasswordResetUseCase(func() *coreauth.EmailTokenService { return svc })

	_, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{Token: "whatever.token", NewPassword: "short"})
	if err == nil {
		t.Fatal("expected a validation error for a too-short password")
	}
}

func TestInviteUserUseCase_Execute_InvalidRole_ReturnsValidationError(t *testing.T) {
	svc, users, resolver, _ := setupEmailUseCases(t, "127.0.0.1:1")
	usersFn := func() *coreauth.UserService { return users }
	uc := NewInviteUserUseCase(usersFn, func() *coreauth.EmailTokenService { return svc }, resolver, slog.Default())

	_, err := uc.Execute(context.Background(), InviteUserInput{Username: "bob", Email: "bob@example.com", Role: "not-a-role"})
	if err == nil {
		t.Fatal("expected a validation error for an invalid role")
	}
}

func TestInviteUserUseCase_Execute_EmailDisabled_ReturnsErrEmailDisabledWithoutCreatingUser(t *testing.T) {
	_, users, resolver, _ := setupEmailUseCases(t, "127.0.0.1:1")
	usersFn := func() *coreauth.UserService { return users }
	uc := NewInviteUserUseCase(usersFn, func() *coreauth.EmailTokenService { return nil }, resolver, slog.Default())

	_, err := uc.Execute(context.Background(), InviteUserInput{Username: "bob", Email: "bob@example.com", Role: coreauth.RoleViewer})
	if err != coreauth.ErrEmailDisabled {
		t.Fatalf("expected ErrEmailDisabled, got %v", err)
	}
	if _, lookupErr := users.GetUserByUsername("bob"); lookupErr != coreauth.ErrUserNotFound {
		t.Fatalf("expected no user to have been created when email is disabled, lookup returned: %v", lookupErr)
	}
}

func TestConfirmInviteUseCase_Execute_ValidToken_IssuesSession(t *testing.T) {
	srv := startFakeInviteSMTPServer(t)
	svc, users, resolver, authSvc := setupEmailUseCases(t, srv.addr)
	usersFn := func() *coreauth.UserService { return users }
	inviteUC := NewInviteUserUseCase(usersFn, func() *coreauth.EmailTokenService { return svc }, resolver, slog.Default())

	if _, err := inviteUC.Execute(context.Background(), InviteUserInput{Username: "bob", Email: "bob@example.com", Role: coreauth.RoleViewer}); err != nil {
		t.Fatalf("InviteUser Execute returned error: %v", err)
	}

	var emailData string
	select {
	case emailData = <-srv.data:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the invite email to be sent")
	}
	rawToken := extractToken(t, emailData)

	confirmUC := NewConfirmInviteUseCase(func() *coreauth.EmailTokenService { return svc }, authSvc)
	confirmOut, err := confirmUC.Execute(context.Background(), ConfirmInviteInput{Token: rawToken, NewPassword: "a-real-password"})
	if err != nil {
		t.Fatalf("ConfirmInvite Execute returned error: %v", err)
	}
	if confirmOut.Token == nil || confirmOut.Token.Token == "" {
		t.Fatal("expected ConfirmInvite to issue a real session token")
	}
}

func TestConfirmInviteUseCase_Execute_InvalidToken_ReturnsError(t *testing.T) {
	svc, _, _, authSvc := setupEmailUseCases(t, "127.0.0.1:1")
	confirmUC := NewConfirmInviteUseCase(func() *coreauth.EmailTokenService { return svc }, authSvc)

	if _, err := confirmUC.Execute(context.Background(), ConfirmInviteInput{Token: "bogus.token", NewPassword: "a-real-password"}); err != coreauth.ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid, got %v", err)
	}
}
