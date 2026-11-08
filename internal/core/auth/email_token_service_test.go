package auth

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/email"
)

// fakeSMTPServer is a minimal single-connection plaintext SMTP server, just
// enough to let email.Client (SecurityNone) complete a send so
// EmailTokenService's mail-dispatch side effects can be asserted on without
// a real SMTP server. Kept intentionally small — protocol-detail coverage
// (multipart framing, header sanitization, STARTTLS/TLS) already lives in
// internal/core/email's own tests; this only needs to observe "was a message
// sent, and to whom".
type fakeSMTPServer struct {
	addr   string
	rcptTo chan string
}

func startFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake smtp listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	srv := &fakeSMTPServer{addr: ln.Addr().String(), rcptTo: make(chan string, 8)}
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

func (s *fakeSMTPServer) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = fmt.Fprintf(conn, "%s\r\n", line) }
	write("220 fake.smtp.test ESMTP")

	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				write("250 OK: message accepted")
			}
			continue
		}

		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			write("250 fake.smtp.test greets you")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			write("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			s.rcptTo <- line
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

func newTestEmailTokenService(t *testing.T, smtpAddr string) (*EmailTokenService, *UserService, *AuthService) {
	t.Helper()
	storageDir := t.TempDir()

	userStore, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("NewUserStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = userStore.Close() })
	users := NewUserService(userStore)

	sessionStore, err := NewSessionStore(storageDir)
	if err != nil {
		t.Fatalf("NewSessionStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sessionStore.Close() })
	sessions := NewSessionManager(sessionStore, "test-jwt-secret-at-least-32-characters-long", time.Hour, 24*time.Hour)
	authService := NewAuthService(users, sessions, nil)

	tokens, err := NewEmailTokenStore(storageDir)
	if err != nil {
		t.Fatalf("NewEmailTokenStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = tokens.Close() })

	host, portStr, err := net.SplitHostPort(smtpAddr)
	if err != nil {
		t.Fatalf("failed to split smtp addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse smtp port: %v", err)
	}

	mailer := email.NewService(email.Config{
		Host:     host,
		Port:     port,
		From:     "wiki@example.com",
		Security: email.SecurityNone,
		Timeout:  5 * time.Second,
	})

	svc := NewEmailTokenService(tokens, users, authService, mailer, "https://wiki.example.com")
	return svc, users, authService
}

func TestEmailTokenService_RequestPasswordReset_UnknownIdentifier_ReturnsNilWithoutContactingSMTP(t *testing.T) {
	// No listener at all — if RequestPasswordReset tried to send mail for an
	// unknown identifier, this would fail to connect and return an error.
	svc, _, _ := newTestEmailTokenService(t, "127.0.0.1:1")

	if err := svc.RequestPasswordReset(context.Background(), "no-such-user"); err != nil {
		t.Fatalf("expected nil for an unknown identifier, got %v", err)
	}
}

func TestEmailTokenService_RequestPasswordReset_KnownIdentifier_SendsToUsersEmail(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, _ := newTestEmailTokenService(t, srv.addr)

	user, err := users.CreateUser("alice", "alice@example.com", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if err := svc.RequestPasswordReset(context.Background(), user.Username); err != nil {
		t.Fatalf("RequestPasswordReset returned error: %v", err)
	}

	select {
	case rcpt := <-srv.rcptTo:
		if !strings.Contains(rcpt, "alice@example.com") {
			t.Fatalf("expected RCPT TO to contain the user's email, got %q", rcpt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the fake SMTP server to receive RCPT TO")
	}
}

func TestEmailTokenService_ConfirmPasswordReset_RevokesExistingSessions(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, authService := newTestEmailTokenService(t, srv.addr)

	user, err := users.CreateUser("alice", "alice@example.com", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	// IssueSessionForUser establishes a pre-reset session the same way a real
	// login would, exercised only through AuthService's public surface (no
	// direct SessionStore access) so this test tracks the actual composition
	// EmailTokenService uses.
	preResetToken, err := authService.IssueSessionForUser(user.ID)
	if err != nil {
		t.Fatalf("IssueSessionForUser returned error: %v", err)
	}

	rawToken, err := svc.tokens.Issue(user.ID, PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, err := svc.ConfirmPasswordReset(rawToken, "a-new-password"); err != nil {
		t.Fatalf("ConfirmPasswordReset returned error: %v", err)
	}

	if _, err := authService.RefreshToken(preResetToken.RefreshToken); err == nil {
		t.Fatal("expected the pre-reset session's refresh token to be revoked")
	}

	if _, err := users.DoesIDAndPasswordMatch(user.ID, "a-new-password"); err != nil {
		t.Fatalf("expected the new password to authenticate, got error: %v", err)
	}
}

func TestEmailTokenService_ConfirmPasswordReset_TokenIsSingleUse(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, _ := newTestEmailTokenService(t, srv.addr)

	user, err := users.CreateUser("alice", "alice@example.com", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	rawToken, err := svc.tokens.Issue(user.ID, PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, err := svc.ConfirmPasswordReset(rawToken, "first-new-password"); err != nil {
		t.Fatalf("first ConfirmPasswordReset returned error: %v", err)
	}
	if _, err := svc.ConfirmPasswordReset(rawToken, "second-new-password"); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid on token reuse, got %v", err)
	}
}

func TestEmailTokenService_ConfirmInvite_ClearsMustSetPasswordAndAuthenticates(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, _ := newTestEmailTokenService(t, srv.addr)

	invited, err := users.InviteUser("bob", "bob@example.com", RoleViewer)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	rawToken, err := svc.tokens.Issue(invited.ID, PurposeInvite, InviteTokenTTL)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	user, err := svc.ConfirmInvite(rawToken, "a-real-password")
	if err != nil {
		t.Fatalf("ConfirmInvite returned error: %v", err)
	}
	if user.MustSetPassword {
		t.Fatal("expected MustSetPassword false after ConfirmInvite")
	}

	if _, err := users.DoesIDAndPasswordMatch(invited.ID, "a-real-password"); err != nil {
		t.Fatalf("expected the new password to authenticate, got error: %v", err)
	}
}

func TestEmailTokenService_ConfirmInvite_RejectsPasswordResetToken(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, _ := newTestEmailTokenService(t, srv.addr)

	invited, err := users.InviteUser("bob", "bob@example.com", RoleViewer)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	// A reset-purpose token for the same user must not be accepted by the
	// invite-accept path — purpose isolation, defense in depth.
	rawToken, err := svc.tokens.Issue(invited.ID, PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, err := svc.ConfirmInvite(rawToken, "a-real-password"); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for a purpose-mismatched token, got %v", err)
	}
}

// An invite token that was issued but not used must stop working once the
// invite is completed some other way, otherwise it can overwrite the
// password of an account that is already in use. Tokens are minted with
// tokens.Issue directly so this pins ConfirmInvite's own check, independent
// of IssueInvite revoking earlier tokens.
func TestEmailTokenService_ConfirmInvite_RejectsTokenAfterPasswordSet(t *testing.T) {
	tests := []struct {
		name     string
		complete func(t *testing.T, svc *EmailTokenService, users *UserService, userID string) string
	}{
		{
			name: "accepted with another invite token",
			complete: func(t *testing.T, svc *EmailTokenService, _ *UserService, userID string) string {
				t.Helper()
				other, err := svc.tokens.Issue(userID, PurposeInvite, InviteTokenTTL)
				if err != nil {
					t.Fatalf("Issue returned error: %v", err)
				}
				if _, err := svc.ConfirmInvite(other, "invitee-password"); err != nil {
					t.Fatalf("ConfirmInvite with the other token returned error: %v", err)
				}
				return "invitee-password"
			},
		},
		{
			name: "password set by an admin",
			complete: func(t *testing.T, _ *EmailTokenService, users *UserService, userID string) string {
				t.Helper()
				if _, err := users.UpdateUser(userID, "bob", "bob@example.com", "admin-set-password", ""); err != nil {
					t.Fatalf("UpdateUser returned error: %v", err)
				}
				return "admin-set-password"
			},
		},
		{
			name: "password set through a reset link",
			complete: func(t *testing.T, svc *EmailTokenService, _ *UserService, userID string) string {
				t.Helper()
				reset, err := svc.tokens.Issue(userID, PurposePasswordReset, PasswordResetTokenTTL)
				if err != nil {
					t.Fatalf("Issue returned error: %v", err)
				}
				if _, err := svc.ConfirmPasswordReset(reset, "reset-password"); err != nil {
					t.Fatalf("ConfirmPasswordReset returned error: %v", err)
				}
				return "reset-password"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, users, _ := newTestEmailTokenService(t, "127.0.0.1:1")

			invited, err := users.InviteUser("bob", "bob@example.com", RoleEditor)
			if err != nil {
				t.Fatalf("InviteUser returned error: %v", err)
			}
			unused, err := svc.tokens.Issue(invited.ID, PurposeInvite, InviteTokenTTL)
			if err != nil {
				t.Fatalf("Issue returned error: %v", err)
			}

			current := tt.complete(t, svc, users, invited.ID)

			if _, err := svc.ConfirmInvite(unused, "later-password"); err != ErrEmailTokenInvalid {
				t.Fatalf("expected ErrEmailTokenInvalid for an invite that is already completed, got %v", err)
			}
			if _, err := users.DoesIDAndPasswordMatch(invited.ID, current); err != nil {
				t.Fatalf("expected the current password to still authenticate, got error: %v", err)
			}
		})
	}
}

// An invite token that outlives its user (the admin deleted the pending
// account) must be rejected like any invalid link, not surface a lookup error.
func TestEmailTokenService_ConfirmInvite_DeletedUser_ReturnsInvalid(t *testing.T) {
	svc, users, _ := newTestEmailTokenService(t, "127.0.0.1:1")

	invited, err := users.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}
	rawToken, err := svc.tokens.Issue(invited.ID, PurposeInvite, InviteTokenTTL)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if err := users.DeleteUser(invited.ID); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}

	if out, err := svc.ConfirmInvite(rawToken, "invitee-password"); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for a deleted user, got out=%v err=%v", out, err)
	}
}

// A resent invite retires the earlier invite link but must leave an open
// password-reset link of the same user working.
func TestEmailTokenService_IssueInvite_KeepsOpenPasswordResetToken(t *testing.T) {
	srv := startFakeSMTPServer(t)
	svc, users, _ := newTestEmailTokenService(t, srv.addr)

	invited, err := users.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}
	earlierInvite, err := svc.tokens.Issue(invited.ID, PurposeInvite, InviteTokenTTL)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	reset, err := svc.tokens.Issue(invited.ID, PurposePasswordReset, PasswordResetTokenTTL)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if err := svc.IssueInvite(context.Background(), invited); err != nil {
		t.Fatalf("IssueInvite returned error: %v", err)
	}

	if _, err := svc.tokens.Resolve(earlierInvite, PurposeInvite); err != ErrEmailTokenInvalid {
		t.Fatalf("expected the earlier invite token to be retired, got %v", err)
	}
	if _, err := svc.tokens.Resolve(reset, PurposePasswordReset); err != nil {
		t.Fatalf("expected the password-reset token to stay valid, got %v", err)
	}
}

// Two different open invite tokens of the same user, confirmed at the same
// time, must not both set a password.
func TestEmailTokenService_ConfirmInvite_ConcurrentTokens_OnlyOneWinner(t *testing.T) {
	svc, users, _ := newTestEmailTokenService(t, "127.0.0.1:1")

	invited, err := users.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	const attempts = 5
	rawTokens := make([]string, attempts)
	for i := range rawTokens {
		if rawTokens[i], err = svc.tokens.Issue(invited.ID, PurposeInvite, InviteTokenTTL); err != nil {
			t.Fatalf("Issue returned error: %v", err)
		}
	}

	var wg sync.WaitGroup
	successes := make([]bool, attempts)
	for i := range rawTokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.ConfirmInvite(rawTokens[i], fmt.Sprintf("password-number-%d", i))
			successes[i] = err == nil
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, ok := range successes {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly 1 successful ConfirmInvite out of %d concurrent attempts, got %d", attempts, wins)
	}
}
