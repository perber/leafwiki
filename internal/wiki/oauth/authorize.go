package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	gooauth "github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	oauthserver "github.com/go-oauth2/oauth2/v4/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

func (r *Routes) handleAuthorize(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		redirectURI, state, err := r.validateAuthorizeRedirectTarget(c.Request)
		if err != nil {
			writeOAuthBadRequest(c, err)
			return
		}

		req, err := r.service.server.ValidationAuthorizeRequest(c.Request)
		if err != nil {
			r.redirectAuthorizeError(c, redirectURI, state, err)
			return
		}
		if err := r.validateAuthorizeRequest(c.Request, req, ctx.Opts.BasePath); err != nil {
			r.redirectAuthorizeError(c, req.RedirectURI, req.State, err)
			return
		}

		user := r.currentWebUser(c, ctx)
		if user == nil {
			loginURL := ctx.Opts.BasePath + "/login"
			original := absoluteRequestURL(c.Request)
			c.Redirect(http.StatusFound, loginURL+"?returnTo="+url.QueryEscape(original))
			return
		}

		approvalValues, approvalKey, err := authorizeApprovalValues(c.Request)
		if err != nil {
			r.redirectAuthorizeError(c, req.RedirectURI, req.State, errors.ErrInvalidRequest)
			return
		}
		switch c.PostForm("decision") {
		case "approve":
			if !r.service.consumeApproval(c.PostForm("approval_token"), user.ID, approvalKey) {
				writeOAuthBadRequest(c, errors.ErrInvalidRequest)
				return
			}
		case "deny":
			_ = r.service.consumeApproval(c.PostForm("approval_token"), user.ID, approvalKey)
			r.redirectAuthorizeError(c, req.RedirectURI, req.State, errors.ErrAccessDenied)
			return
		default:
			details := r.service.approvalPageData(c.Request, req, ctx.Opts.BasePath)
			token, err := r.service.issueApproval(user.ID, approvalKey, details)
			if err != nil {
				writeOAuthBadRequest(c, err)
				return
			}
			approvalValues.Set("approval_token", token)
			c.Redirect(http.StatusFound, ctx.Opts.BasePath+"/oauth/approve?"+approvalValues.Encode())
			return
		}

		req.UserID = user.ID
		req.Scope = ScopeMCP
		req.AccessTokenExp = r.service.accessTTL

		info, err := r.service.server.GetAuthorizeToken(c.Request.Context(), req)
		if err != nil {
			writeOAuthBadRequest(c, err)
			return
		}
		targetURI, err := r.service.server.GetRedirectURI(req, r.service.server.GetAuthorizeData(req.ResponseType, info))
		if err != nil {
			writeOAuthBadRequest(c, err)
			return
		}
		c.Redirect(http.StatusFound, targetURI)
	}
}

func (r *Routes) currentWebUser(c *gin.Context, ctx httpinternal.RouterContext) *coreauth.User {
	user, err := authmw.ResolveRequestUser(c, r.service.auth, ctx.AuthCookies, false)
	if err != nil {
		return nil
	}
	return user
}

func (r *Routes) validateAuthorizeRequest(req *http.Request, ar *oauthserver.AuthorizeRequest, basePath string) error {
	client, ok := r.service.client(ar.ClientID)
	if !ok {
		return fmt.Errorf("unknown oauth client")
	}
	if !stringSliceContains(client.ResponseTypes, string(ar.ResponseType)) {
		return errors.ErrUnsupportedResponseType
	}
	if !clientScopeAllowed(client, ar.Scope) {
		return errors.ErrInvalidScope
	}
	if ar.CodeChallenge == "" || ar.CodeChallengeMethod != gooauth.CodeChallengeS256 {
		return errors.ErrInvalidRequest
	}
	if err := validateLoopbackRedirectURI(ar.RedirectURI); err != nil {
		return err
	}
	if !clientRedirectURIAllowed(client, ar.RedirectURI) {
		return errors.ErrInvalidRequest
	}
	if err := validateAuthorizeResource(req, basePath); err != nil {
		return errors.ErrInvalidRequest
	}
	return nil
}

func validateAuthorizeResource(req *http.Request, basePath string) error {
	if err := req.ParseForm(); err != nil {
		return err
	}
	resources, ok := req.Form["resource"]
	if !ok || len(resources) == 0 {
		return nil
	}
	if len(resources) != 1 {
		return fmt.Errorf("resource must be supplied once")
	}
	if resources[0] != MCPResourceURL(req, basePath) {
		return fmt.Errorf("resource must match MCP resource URL")
	}
	return nil
}

func (r *Routes) validateAuthorizeRedirectTarget(req *http.Request) (string, string, error) {
	clientID := req.FormValue("client_id")
	client, ok := r.service.client(clientID)
	if !ok {
		return "", "", fmt.Errorf("unknown oauth client")
	}
	redirectURI := req.FormValue("redirect_uri")
	if err := validateLoopbackRedirectURI(redirectURI); err != nil {
		return "", "", err
	}
	if !clientRedirectURIAllowed(client, redirectURI) {
		return "", "", fmt.Errorf("redirect_uri is not registered for this client")
	}
	return redirectURI, req.FormValue("state"), nil
}

func (r *Routes) redirectAuthorizeError(c *gin.Context, redirectURI, state string, err error) {
	data, _, _ := r.service.server.GetErrorData(err)
	target, redirectErr := r.service.server.GetRedirectURI(&oauthserver.AuthorizeRequest{
		RedirectURI:  redirectURI,
		ResponseType: gooauth.Code,
		State:        state,
	}, data)
	if redirectErr != nil {
		writeOAuthBadRequest(c, redirectErr)
		return
	}
	c.Redirect(http.StatusFound, target)
}

func requestedScopeAllowed(scope string) bool {
	if strings.TrimSpace(scope) == "" {
		return true
	}
	for _, value := range strings.Fields(scope) {
		if value != ScopeMCP {
			return false
		}
	}
	return true
}

func validateLoopbackRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return fmt.Errorf("invalid redirect_uri")
	}
	if u.Scheme != "http" {
		return fmt.Errorf("redirect_uri must use http")
	}
	if u.Port() == "" {
		return fmt.Errorf("redirect_uri must include an explicit port")
	}
	if u.Fragment != "" {
		return fmt.Errorf("redirect_uri must not include a fragment")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	return fmt.Errorf("redirect_uri must be loopback")
}

func clientRedirectURIAllowed(client registeredClient, redirectURI string) bool {
	if len(client.RedirectURIs) == 0 {
		return true
	}
	return stringSliceContains(client.RedirectURIs, redirectURI)
}

func clientScopeAllowed(client registeredClient, scope string) bool {
	if !requestedScopeAllowed(scope) {
		return false
	}
	if strings.TrimSpace(scope) == "" || client.Scope == "" {
		return true
	}
	allowed := strings.Fields(client.Scope)
	for _, value := range strings.Fields(scope) {
		if !stringSliceContains(allowed, value) {
			return false
		}
	}
	return true
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
