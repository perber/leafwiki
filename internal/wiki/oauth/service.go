package oauth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	gooauth "github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	oauthserver "github.com/go-oauth2/oauth2/v4/server"
	oauthstore "github.com/go-oauth2/oauth2/v4/store"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

const (
	ClientID    = "leafwiki-local-mcp"
	ScopeMCP    = "leafwiki:mcp"
	approvalTTL = 10 * time.Minute
)

// Service owns the local in-memory OAuth server used by authenticated MCP.
type Service struct {
	auth        *coreauth.AuthService
	users       *coreauth.UserService
	manager     *manage.Manager
	server      *oauthserver.Server
	clientStore *oauthstore.ClientStore
	accessTTL   time.Duration
	refreshTTL  time.Duration
	clientsMu   sync.RWMutex
	clients     map[string]registeredClient
	approvalMu  sync.Mutex
	approvals   map[string]oauthApproval
}

type ServiceConfig struct {
	AuthService         *coreauth.AuthService
	UserService         *coreauth.UserService
	AccessTokenTimeout  time.Duration
	RefreshTokenTimeout time.Duration
}

func NewService(cfg ServiceConfig) (*Service, error) {
	manager := manage.NewDefaultManager()
	manager.SetValidateURIHandler(func(_, redirectURI string) error {
		return validateLoopbackRedirectURI(redirectURI)
	})

	tokenStore, err := newTokenStore()
	if err != nil {
		return nil, fmt.Errorf("create oauth token store: %w", err)
	}
	manager.MapTokenStorage(tokenStore)

	clientStore := oauthstore.NewClientStore()
	if err := clientStore.Set(ClientID, &models.Client{ID: ClientID, Public: true}); err != nil {
		return nil, fmt.Errorf("register oauth client: %w", err)
	}
	manager.MapClientStorage(clientStore)

	service := &Service{
		auth:        cfg.AuthService,
		users:       cfg.UserService,
		manager:     manager,
		clientStore: clientStore,
		accessTTL:   cfg.AccessTokenTimeout,
		refreshTTL:  cfg.RefreshTokenTimeout,
		clients: map[string]registeredClient{
			ClientID: {
				ClientName:    "LeafWiki local MCP",
				GrantTypes:    []string{string(gooauth.AuthorizationCode), string(gooauth.Refreshing)},
				ResponseTypes: []string{string(gooauth.Code)},
				Scope:         ScopeMCP,
			},
		},
		approvals: map[string]oauthApproval{},
	}
	manager.MapAccessGenerate(clientGrantAccessGenerate{
		base:    generates.NewAccessGenerate(),
		service: service,
	})
	service.configureTokenLifetimes()

	serverCfg := oauthserver.NewConfig()
	serverCfg.AllowedResponseTypes = []gooauth.ResponseType{gooauth.Code}
	serverCfg.AllowedGrantTypes = []gooauth.GrantType{gooauth.AuthorizationCode, gooauth.Refreshing}
	serverCfg.AllowedCodeChallengeMethods = []gooauth.CodeChallengeMethod{gooauth.CodeChallengeS256}
	serverCfg.ForcePKCE = true

	srv := oauthserver.NewServer(serverCfg, manager)
	srv.SetClientInfoHandler(oauthserver.ClientFormHandler)
	srv.SetClientAuthorizedHandler(func(clientID string, grant gooauth.GrantType) (bool, error) {
		return service.clientAllowsGrant(clientID, grant), nil
	})
	srv.SetClientScopeHandler(func(tgr *gooauth.TokenGenerateRequest) (bool, error) {
		return requestedScopeAllowed(tgr.Scope), nil
	})
	srv.SetRefreshingScopeHandler(func(tgr *gooauth.TokenGenerateRequest, oldScope string) (bool, error) {
		if !requestedScopeAllowed(tgr.Scope) {
			return false, nil
		}
		return tgr.Scope == "" || tgr.Scope == oldScope, nil
	})
	srv.SetRefreshingValidationHandler(func(ti gooauth.TokenInfo) (bool, error) {
		if cfg.UserService == nil {
			return false, errors.ErrInvalidGrant
		}
		if _, err := cfg.UserService.GetUserByID(ti.GetUserID()); err != nil {
			return false, errors.ErrInvalidGrant
		}
		return true, nil
	})
	service.server = srv

	return service, nil
}

func newTokenStore() (gooauth.TokenStore, error) {
	// go-oauth2's in-memory token store uses BuntDB internally; keep that MVP
	// dependency behind one helper if the storage choice changes later.
	return oauthstore.NewMemoryTokenStore()
}

type clientGrantAccessGenerate struct {
	base    gooauth.AccessGenerate
	service *Service
}

func (g clientGrantAccessGenerate) Token(ctx context.Context, data *gooauth.GenerateBasic, isGenRefresh bool) (string, string, error) {
	if isGenRefresh && g.service != nil && data != nil && data.Client != nil && !g.service.clientAllowsGrant(data.Client.GetID(), gooauth.Refreshing) {
		isGenRefresh = false
	}
	return g.base.Token(ctx, data, isGenRefresh)
}

func (s *Service) configureTokenLifetimes() {
	s.manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    s.accessTTL,
		RefreshTokenExp:   s.refreshTTL,
		IsGenerateRefresh: true,
	})
	s.manager.SetRefreshTokenCfg(&manage.RefreshingConfig{
		AccessTokenExp:     s.accessTTL,
		RefreshTokenExp:    s.refreshTTL,
		IsGenerateRefresh:  true,
		IsRemoveAccess:     true,
		IsRemoveRefreshing: true,
	})
}

func (s *Service) VerifyBearerToken(ctx context.Context, token string, req *http.Request) (*sdkauth.TokenInfo, error) {
	if s == nil || s.server == nil || s.users == nil {
		return nil, fmt.Errorf("%w: oauth unavailable", sdkauth.ErrInvalidToken)
	}

	tokenReq := req.Clone(ctx)
	tokenReq.Header = tokenReq.Header.Clone()
	tokenReq.Header.Set("Authorization", "Bearer "+token)

	info, err := s.server.ValidationBearerToken(tokenReq)
	if err != nil || info == nil {
		if err == nil {
			err = errors.ErrInvalidAccessToken
		}
		return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
	}

	user, err := s.users.GetUserByID(info.GetUserID())
	if err != nil {
		return nil, fmt.Errorf("%w: user not found", sdkauth.ErrInvalidToken)
	}

	return &sdkauth.TokenInfo{
		UserID:     user.ID,
		Scopes:     strings.Fields(info.GetScope()),
		Expiration: info.GetAccessCreateAt().Add(info.GetAccessExpiresIn()),
	}, nil
}

func (s *Service) client(clientID string) (registeredClient, bool) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	client, ok := s.clients[clientID]
	return client, ok
}

func (s *Service) clientAllowsGrant(clientID string, grant gooauth.GrantType) bool {
	client, ok := s.client(clientID)
	return ok && stringSliceContains(client.GrantTypes, string(grant))
}

func (s *Service) validateRefreshRequest(req *http.Request) error {
	if req.Method != http.MethodPost {
		return nil
	}
	if err := req.ParseForm(); err != nil {
		return errors.ErrInvalidRequest
	}
	if req.FormValue("grant_type") != string(gooauth.Refreshing) {
		return nil
	}
	clientID := strings.TrimSpace(req.FormValue("client_id"))
	refreshToken := strings.TrimSpace(req.FormValue("refresh_token"))
	if clientID == "" || refreshToken == "" {
		return errors.ErrInvalidRequest
	}
	info, err := s.manager.LoadRefreshToken(req.Context(), refreshToken)
	if err != nil {
		return errors.ErrInvalidGrant
	}
	if info.GetClientID() != clientID {
		return errors.ErrInvalidGrant
	}
	if !s.clientAllowsGrant(info.GetClientID(), gooauth.Refreshing) {
		return errors.ErrInvalidGrant
	}
	return nil
}
