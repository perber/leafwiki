package email

import (
	"context"
	"fmt"
)

// Service is the mail-sending façade used by internal/core/auth's
// EmailTokenService. It is only constructed when Config.Enabled() and auth
// are both true (see wiki.initEmail) — nil-checked everywhere it's consumed,
// same as this codebase's other optional-feature services.
type Service struct {
	cfg    Config
	client *Client
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg, client: NewClient(cfg)}
}

func (s *Service) SendPasswordResetEmail(ctx context.Context, to, link string) error {
	html, text, err := renderPasswordReset(link)
	if err != nil {
		return fmt.Errorf("render password reset email: %w", err)
	}
	return s.send(ctx, to, "Reset your LeafWiki password", html, text)
}

func (s *Service) SendInviteEmail(ctx context.Context, to, link string) error {
	html, text, err := renderInvite(link)
	if err != nil {
		return fmt.Errorf("render invite email: %w", err)
	}
	return s.send(ctx, to, "You've been invited to LeafWiki", html, text)
}

// send is not the swallow point for delivery failures — callers (the
// forgot-password fire-and-forget goroutine, the synchronous invite-send
// handler) decide whether/how to log or surface the error, per ADR-0008's
// "log once, at the swallow point" — so this deliberately does not log.
func (s *Service) send(ctx context.Context, to, subject, html, text string) error {
	sendCtx, cancel := context.WithTimeout(ctx, s.cfg.TimeoutOrDefault())
	defer cancel()

	return s.client.Send(sendCtx, Message{To: to, Subject: subject, HTML: html, Text: text})
}
