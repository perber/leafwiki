package http_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/email"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

// inviteMailbox is a minimal plaintext SMTP server that hands every message
// body it receives to the test, so a router test can follow the link in an
// invite or password-reset email.
type inviteMailbox struct {
	addr     string
	messages chan string
}

func startInviteMailbox(t *testing.T) *inviteMailbox {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake smtp listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	mb := &inviteMailbox{addr: ln.Addr().String(), messages: make(chan string, 8)}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go mb.serve(conn)
		}
	}()
	return mb
}

func (mb *inviteMailbox) serve(conn net.Conn) {
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
				mb.messages <- strings.Join(lines, "\n")
				lines = nil
				write("250 OK: message accepted")
				continue
			}
			lines = append(lines, line)
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "MAIL FROM:"), strings.HasPrefix(upper, "RCPT TO:"):
			write("250 OK")
		case upper == "DATA":
			inData = true
			write("354 End data with <CR><LF>.<CR><LF>")
		case upper == "QUIT":
			write("221 Bye")
			return
		default:
			write("500 unrecognized command")
		}
	}
}

var mailedTokenRe = regexp.MustCompile(`token=([A-Za-z0-9._-]+)`)

// nextToken waits for the next email and returns the token from its link.
func (mb *inviteMailbox) nextToken(t *testing.T) string {
	t.Helper()
	select {
	case msg := <-mb.messages:
		m := mailedTokenRe.FindStringSubmatch(msg)
		if m == nil {
			t.Fatalf("could not find a token in the sent email:\n%s", msg)
		}
		return m[1]
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an email to be sent")
		return ""
	}
}

func postJSON(router http.Handler, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// An invite link that was never used must stop working once the invited
// account has a real password, whichever way the password was set: the
// accept endpoint answers 422 auth_token_invalid like any invalid link, and
// the password the account relies on keeps working.
func TestInviteAccept_StaleTokenReturnsInvalid(t *testing.T) {
	tests := []struct {
		name string
		// setPassword completes the invite without the invite link and
		// returns the password the account now uses.
		setPassword func(t *testing.T, router http.Handler, mb *inviteMailbox, userID string) string
	}{
		{
			name: "admin sets the password",
			setPassword: func(t *testing.T, router http.Handler, _ *inviteMailbox, userID string) string {
				t.Helper()
				update := `{"username": "bob", "email": "bob@example.com", "password": "adminsetpassword"}`
				if rec := authenticatedRequest(t, router, http.MethodPut, "/api/users/"+userID, strings.NewReader(update)); rec.Code != http.StatusOK {
					t.Fatalf("Expected 200 OK for admin password update, got %d - %s", rec.Code, rec.Body.String())
				}
				return "adminsetpassword"
			},
		},
		{
			name: "invitee uses a password reset link",
			setPassword: func(t *testing.T, router http.Handler, mb *inviteMailbox, _ string) string {
				t.Helper()
				if rec := postJSON(router, "/api/auth/password/forgot", `{"identifier": "bob@example.com"}`); rec.Code >= 300 {
					t.Fatalf("Expected forgot-password to succeed, got %d - %s", rec.Code, rec.Body.String())
				}
				reset := mb.nextToken(t)
				body, _ := json.Marshal(map[string]string{"token": reset, "newPassword": "resetpassword"})
				if rec := postJSON(router, "/api/auth/password/reset", string(body)); rec.Code >= 300 {
					t.Fatalf("Expected password reset to succeed, got %d - %s", rec.Code, rec.Body.String())
				}
				return "resetpassword"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := startInviteMailbox(t)
			host, portStr, err := net.SplitHostPort(mb.addr)
			if err != nil {
				t.Fatalf("failed to split smtp addr: %v", err)
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				t.Fatalf("failed to parse smtp port: %v", err)
			}
			w, err := wiki.NewWiki(&wiki.WikiOptions{
				StorageDir:          t.TempDir(),
				AdminPassword:       "adminpassword",
				JWTSecret:           "secretkey",
				AccessTokenTimeout:  15 * time.Minute,
				RefreshTokenTimeout: 7 * 24 * time.Hour,
				SMTP: email.Config{
					Host:      host,
					Port:      port,
					From:      "wiki@example.com",
					Security:  email.SecurityNone,
					Timeout:   5 * time.Second,
					PublicURL: "https://wiki.example.com",
				},
			})
			if err != nil {
				t.Fatalf("Failed to create wiki instance: %v", err)
			}
			defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
			router := createRouterTestInstance(w, t)

			invite := `{"username": "bob", "email": "bob@example.com", "role": "editor"}`
			rec := authenticatedRequest(t, router, http.MethodPost, "/api/users/invite", strings.NewReader(invite))
			if rec.Code != http.StatusCreated {
				t.Fatalf("Expected 201 Created for invite, got %d - %s", rec.Code, rec.Body.String())
			}
			var created struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.User.ID == "" {
				t.Fatalf("Expected the invited user in the response, got %s (err %v)", rec.Body.String(), err)
			}
			inviteToken := mb.nextToken(t)

			current := tt.setPassword(t, router, mb, created.User.ID)

			body, _ := json.Marshal(map[string]string{"token": inviteToken, "newPassword": "laterpassword"})
			rec = postJSON(router, "/api/auth/invite/accept", string(body))
			if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"auth_token_invalid"`) {
				t.Fatalf("Expected 422 auth_token_invalid for the stale invite link, got %d - %s", rec.Code, rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatalf("Expected no auth cookies for a rejected invite link, got %v", rec.Result().Cookies())
			}
			if code := loginStatus(t, router, "bob", current); code != http.StatusOK {
				t.Fatalf("Expected the current password to still log in, got %d", code)
			}
		})
	}
}
