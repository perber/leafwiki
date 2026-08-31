package backup

import (
	"context"
	"errors"
	"fmt"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

// TestRemoteTimeout bounds a single connectivity check. Kept short so the
// settings "test connection" request can't hang an admin's browser tab.
const TestRemoteTimeout = 30 * time.Second

// TestRemote verifies that cfg's remote is reachable and its credentials are
// accepted, via an in-memory `git ls-remote` — no working tree, nothing
// written to disk. It backs both the settings UI's explicit "test connection"
// button and the implicit check performed on save.
//
// An empty-but-reachable remote (no commits yet) is treated as success: that
// is a perfectly good backup target, the first push just creates the branch.
func TestRemote(ctx context.Context, cfg Config) error {
	if cfg.RemoteURL == "" {
		return errors.New("no remote repository URL configured")
	}

	auth, err := buildAuth(cfg)
	if err != nil {
		return fmt.Errorf("could not build credentials for the remote: %w", err)
	}

	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{cfg.RemoteURL},
	})

	ctx, cancel := context.WithTimeout(ctx, TestRemoteTimeout)
	defer cancel()

	if _, err := rem.ListContext(ctx, &gogit.ListOptions{Auth: auth}); err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}
		return describeRemoteErr(err)
	}
	return nil
}

// describeRemoteErr turns go-git's transport sentinels into messages an admin
// can act on. The original error is wrapped so callers/tests can still match
// on it.
func describeRemoteErr(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("timed out after %s connecting to the remote — check the URL and network access: %w", TestRemoteTimeout, err)
	case errors.Is(err, transport.ErrAuthenticationRequired),
		errors.Is(err, transport.ErrAuthorizationFailed):
		return fmt.Errorf("the remote rejected the credentials — check the SSH key or username/token: %w", err)
	case errors.Is(err, transport.ErrRepositoryNotFound):
		return fmt.Errorf("no repository found at that URL — create it on the remote first, or fix the URL: %w", err)
	default:
		return fmt.Errorf("could not reach the remote: %w", err)
	}
}