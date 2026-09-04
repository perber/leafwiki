package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/perber/wiki/internal/core/email"
)

// EmailTokenService issues and consumes password-reset and invite tokens. It
// composes UserService (identity), EmailTokenStore (token persistence),
// AuthService (post-reset session revocation — reuses
// AuthService.RevokeAllUserSessions rather than taking a raw *SessionStore),
// and email.Service (delivery). It is only constructed when SMTP is
// configured and auth is enabled — see wiki.initEmail; nil-checked at every
// call site that resolves it.
type EmailTokenService struct {
	tokens    *EmailTokenStore
	users     *UserService
	auth      *AuthService
	mailer    *email.Service
	publicURL string
	log       *slog.Logger

	// wg tracks in-flight RequestPasswordResetAsync sends so Close can drain
	// them instead of letting a shutdown race a still-running SMTP send.
	wg sync.WaitGroup
}

func NewEmailTokenService(tokens *EmailTokenStore, users *UserService, authService *AuthService, mailer *email.Service, publicURL string) *EmailTokenService {
	return &EmailTokenService{
		tokens:    tokens,
		users:     users,
		auth:      authService,
		mailer:    mailer,
		publicURL: strings.TrimSuffix(publicURL, "/"),
		log:       slog.Default().With("component", "EmailTokenService"),
	}
}

// RequestPasswordReset issues a token and emails a reset link to identifier
// (username or email) if it resolves to a user, but always returns nil
// either way — this method is itself a defense-in-depth no-op on an unknown
// identifier; the actual enumeration-timing boundary callers must uphold is
// at the HTTP layer (internal/wiki/auth's RequestPasswordResetUseCase),
// which must respond identically and only dispatch this call from a
// fire-and-forget goroutine so its own timing never depends on whether a
// user existed or on SMTP round-trip time.
func (s *EmailTokenService) RequestPasswordReset(ctx context.Context, identifier string) error {
	user, err := s.users.GetUserByIdentifier(identifier)
	if err != nil {
		return nil
	}

	rawToken, err := s.tokens.Issue(user.ID, PurposePasswordReset, PasswordResetTokenTTL)
	if err != nil {
		return fmt.Errorf("issue password reset token: %w", err)
	}

	link := s.buildLink("/reset-password", rawToken)
	if err := s.mailer.SendPasswordResetEmail(ctx, user.Email, link); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// RequestPasswordResetAsync starts RequestPasswordReset on a background
// goroutine and returns immediately, tracked by wg so Close can drain it —
// this is the actual enumeration-timing boundary: the HTTP handler calling
// this (RequestPasswordResetUseCase) must respond identically whether or not
// identifier resolved to a user, so it cannot wait for (or report the
// outcome of) the send itself. Failures are logged here, at this swallow
// point, per ADR-0008.
func (s *EmailTokenService) RequestPasswordResetAsync(identifier string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.RequestPasswordReset(context.Background(), identifier); err != nil {
			s.log.Warn("failed to send password reset email", "error", err)
		}
	}()
}

// Close waits for any in-flight RequestPasswordResetAsync sends to finish.
// It does not close the underlying EmailTokenStore — that has its own
// lifetime and Close method, managed separately by wiki.Wiki.Close.
func (s *EmailTokenService) Close() {
	s.wg.Wait()
}

// ConfirmPasswordReset consumes rawToken, sets newPassword, and revokes every
// existing session for the user — deliberate: a reset implies the account
// may have been compromised, so pre-reset sessions must not survive it
// (unlike ChangeOwnPassword, which does not revoke sessions today — that is
// an intentional scope boundary, not an inconsistency to fix here).
func (s *EmailTokenService) ConfirmPasswordReset(rawToken, newPassword string) (*User, error) {
	tok, err := s.tokens.Resolve(rawToken, PurposePasswordReset)
	if err != nil {
		return nil, err
	}
	if err := s.tokens.Consume(tok.ID); err != nil {
		return nil, err
	}

	if err := s.users.UpdatePassword(tok.UserID, newPassword); err != nil {
		return nil, err
	}
	if err := s.auth.RevokeAllUserSessions(tok.UserID); err != nil {
		s.log.Warn("failed to revoke sessions after password reset", "userID", tok.UserID, "error", err)
	}

	return s.users.GetUserByID(tok.UserID)
}

// IssueInvite mints an invite token for an already-created user and emails
// it. Unlike RequestPasswordReset, the caller (InviteUserUseCase) already
// knows the user exists — it just created them — so no enumeration concern
// applies here.
func (s *EmailTokenService) IssueInvite(ctx context.Context, user *User) error {
	rawToken, err := s.tokens.Issue(user.ID, PurposeInvite, InviteTokenTTL)
	if err != nil {
		return fmt.Errorf("issue invite token: %w", err)
	}

	link := s.buildLink("/accept-invite", rawToken)
	return s.mailer.SendInviteEmail(ctx, user.Email, link)
}

// ConfirmInvite consumes rawToken, sets the user's real password, clears
// MustSetPassword, and returns the user so the caller can auto-issue a
// session (a freshly invited user has no prior sessions to worry about,
// unlike ConfirmPasswordReset).
func (s *EmailTokenService) ConfirmInvite(rawToken, newPassword string) (*User, error) {
	tok, err := s.tokens.Resolve(rawToken, PurposeInvite)
	if err != nil {
		return nil, err
	}
	if err := s.tokens.Consume(tok.ID); err != nil {
		return nil, err
	}

	if err := s.users.CompleteInvite(tok.UserID, newPassword); err != nil {
		return nil, err
	}

	return s.users.GetUserByID(tok.UserID)
}

func (s *EmailTokenService) buildLink(path, rawToken string) string {
	return fmt.Sprintf("%s%s?token=%s", s.publicURL, path, rawToken)
}
