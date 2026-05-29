package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	gooauth "github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
)

type registeredClient struct {
	ClientName    string
	RedirectURIs  []string
	GrantTypes    []string
	ResponseTypes []string
	Scope         string
}

type clientRegistrationRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
	ClientSecret            string   `json:"client_secret"`
}

func (r *Routes) handleRegister(c *gin.Context) {
	var req clientRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeRegistrationError(c, "invalid client metadata")
		return
	}
	if req.ClientSecret != "" {
		writeRegistrationError(c, "client_secret is not supported")
		return
	}
	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		writeRegistrationError(c, "only public clients with token_endpoint_auth_method none are supported")
		return
	}
	redirectURIs, err := normalizeRedirectURIs(req.RedirectURIs)
	if err != nil {
		writeRegistrationError(c, err.Error())
		return
	}
	grantTypes, err := normalizeRegistrationGrantTypes(req.GrantTypes)
	if err != nil {
		writeRegistrationError(c, err.Error())
		return
	}
	responseTypes, err := normalizeRegistrationResponseTypes(req.ResponseTypes)
	if err != nil {
		writeRegistrationError(c, err.Error())
		return
	}
	scope, err := normalizeRegistrationScope(req.Scope)
	if err != nil {
		writeRegistrationError(c, err.Error())
		return
	}
	clientID, err := r.service.registerDynamicClient(registeredClient{
		ClientName:    strings.TrimSpace(req.ClientName),
		RedirectURIs:  redirectURIs,
		GrantTypes:    grantTypes,
		ResponseTypes: responseTypes,
		Scope:         scope,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": "failed to register client",
		})
		return
	}

	body := gin.H{
		"client_id":                  clientID,
		"redirect_uris":              redirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                grantTypes,
		"response_types":             responseTypes,
	}
	if clientName := strings.TrimSpace(req.ClientName); clientName != "" {
		body["client_name"] = clientName
	}
	if scope != "" {
		body["scope"] = scope
	}
	c.JSON(http.StatusCreated, body)
}

func (s *Service) registerDynamicClient(client registeredClient) (string, error) {
	for range 8 {
		clientID, err := randomClientID()
		if err != nil {
			return "", err
		}

		s.clientsMu.Lock()
		if _, exists := s.clients[clientID]; exists {
			s.clientsMu.Unlock()
			continue
		}
		if err := s.clientStore.Set(clientID, &models.Client{ID: clientID, Public: true}); err != nil {
			s.clientsMu.Unlock()
			return "", err
		}
		s.clients[clientID] = client
		s.clientsMu.Unlock()
		return clientID, nil
	}
	return "", fmt.Errorf("generate unique oauth client id")
}

func randomClientID() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("create oauth client id: %w", err)
	}
	return "leafwiki-dcr-" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func normalizeRedirectURIs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("redirect_uris is required")
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		redirectURI := strings.TrimSpace(value)
		if redirectURI == "" {
			return nil, fmt.Errorf("redirect_uris must not contain empty values")
		}
		if err := validateLoopbackRedirectURI(redirectURI); err != nil {
			return nil, err
		}
		out = append(out, redirectURI)
	}
	return out, nil
}

func normalizeRegistrationGrantTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{string(gooauth.AuthorizationCode), string(gooauth.Refreshing)}, nil
	}

	hasAuthorizationCode := false
	hasRefreshToken := false
	for _, value := range values {
		switch value {
		case string(gooauth.AuthorizationCode):
			hasAuthorizationCode = true
		case string(gooauth.Refreshing):
			hasRefreshToken = true
		default:
			return nil, fmt.Errorf("unsupported grant_type %q", value)
		}
	}
	if !hasAuthorizationCode {
		return nil, fmt.Errorf("authorization_code grant_type is required")
	}
	out := []string{string(gooauth.AuthorizationCode)}
	if hasRefreshToken {
		out = append(out, string(gooauth.Refreshing))
	}
	return out, nil
}

func normalizeRegistrationResponseTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{string(gooauth.Code)}, nil
	}
	for _, value := range values {
		if value != string(gooauth.Code) {
			return nil, fmt.Errorf("unsupported response_type %q", value)
		}
	}
	return []string{string(gooauth.Code)}, nil
}

func normalizeRegistrationScope(scope string) (string, error) {
	scope = strings.Join(strings.Fields(scope), " ")
	if scope == "" {
		return "", nil
	}
	if !requestedScopeAllowed(scope) {
		return "", fmt.Errorf("unsupported scope")
	}
	return ScopeMCP, nil
}
