package apikeys

import (
	"context"
	"errors"
	"strings"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ErrAPIKeysDisabled is returned when an api-key operation is called while
// authentication is disabled (mirrors wikiauth.ErrAuthDisabled). The HTTP
// routes are already RequireAdmin-gated (which itself hard-blocks when auth
// is disabled), so this should be unreachable in practice — it exists as
// defense in depth against a nil APIKeyService, the same way LoginUseCase
// guards against a nil AuthService rather than relying solely on the
// HTTP-layer guard.
var ErrAPIKeysDisabled = errors.New("api key management is disabled")

// ─── CreateAPIKeyUseCase ─────────────────────────────────────────────────────

type CreateAPIKeyInput struct {
	Name      string
	UserID    string
	Role      string // empty defaults to coreauth.RoleViewer
	ExpiresAt *time.Time
	CreatedBy string
}

type CreateAPIKeyOutput struct {
	Key    *coreauth.APIKey
	Secret string // plaintext token, shown to the caller exactly once
}

type CreateAPIKeyUseCase struct {
	keys *coreauth.APIKeyService
}

func NewCreateAPIKeyUseCase(k *coreauth.APIKeyService) *CreateAPIKeyUseCase {
	return &CreateAPIKeyUseCase{keys: k}
}

func (uc *CreateAPIKeyUseCase) Execute(_ context.Context, in CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
	if uc.keys == nil {
		return nil, ErrAPIKeysDisabled
	}

	ve := sharederrors.NewValidationErrors()
	if strings.TrimSpace(in.Name) == "" {
		ve.Add("name", "Name must not be empty")
	}
	if strings.TrimSpace(in.UserID) == "" {
		ve.Add("userId", "Owning user is required")
	}
	if in.Role != "" && !coreauth.IsValidRole(in.Role) {
		ve.Add("role", "Invalid role")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		ve.Add("expiresAt", "Expiry date must be in the future")
	}
	if ve.HasErrors() {
		return nil, ve
	}

	key, secret, err := uc.keys.CreateAPIKey(coreauth.CreateAPIKeyParams{
		Name:      in.Name,
		UserID:    in.UserID,
		Role:      in.Role,
		ExpiresAt: in.ExpiresAt,
		CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	return &CreateAPIKeyOutput{Key: key, Secret: secret}, nil
}

// ─── ListAPIKeysUseCase ──────────────────────────────────────────────────────

type ListAPIKeysOutput struct {
	Keys []*coreauth.APIKey
}

type ListAPIKeysUseCase struct {
	keys *coreauth.APIKeyService
}

func NewListAPIKeysUseCase(k *coreauth.APIKeyService) *ListAPIKeysUseCase {
	return &ListAPIKeysUseCase{keys: k}
}

func (uc *ListAPIKeysUseCase) Execute(_ context.Context) (*ListAPIKeysOutput, error) {
	if uc.keys == nil {
		return nil, ErrAPIKeysDisabled
	}

	keys, err := uc.keys.ListAPIKeys()
	if err != nil {
		return nil, err
	}
	return &ListAPIKeysOutput{Keys: keys}, nil
}

// ─── RevokeAPIKeyUseCase ─────────────────────────────────────────────────────

type RevokeAPIKeyInput struct{ ID string }

type RevokeAPIKeyUseCase struct {
	keys *coreauth.APIKeyService
}

func NewRevokeAPIKeyUseCase(k *coreauth.APIKeyService) *RevokeAPIKeyUseCase {
	return &RevokeAPIKeyUseCase{keys: k}
}

func (uc *RevokeAPIKeyUseCase) Execute(_ context.Context, in RevokeAPIKeyInput) error {
	if uc.keys == nil {
		return ErrAPIKeysDisabled
	}
	return uc.keys.RevokeAPIKey(in.ID)
}
