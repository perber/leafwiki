package oauth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
)

func (r *Routes) handleAuthorizationServerMetadata(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		issuer := IssuerURL(c.Request, ctx.Opts.BasePath)
		c.JSON(http.StatusOK, gin.H{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/oauth/authorize",
			"token_endpoint":                        issuer + "/oauth/token",
			"registration_endpoint":                 issuer + "/oauth/register",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"code_challenge_methods_supported":      []string{"S256"},
			"scopes_supported":                      []string{ScopeMCP},
			"token_endpoint_auth_methods_supported": []string{"none"},
		})
	}
}

func (r *Routes) handleProtectedResourceMetadata(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"resource":              MCPResourceURL(c.Request, ctx.Opts.BasePath),
			"authorization_servers": []string{IssuerURL(c.Request, ctx.Opts.BasePath)},
			"scopes_supported":      []string{ScopeMCP},
		})
	}
}

func AuthorizationServerMetadataPaths(basePath string) []string {
	paths := []string{AuthorizationServerMetadataPath("")}
	if basePath != "" {
		paths = append(paths, AuthorizationServerMetadataPath(basePath))
	}
	return paths
}

func AuthorizationServerMetadataPath(basePath string) string {
	if basePath == "" {
		return "/.well-known/oauth-authorization-server"
	}
	return "/.well-known/oauth-authorization-server" + basePath
}

func ProtectedResourceMetadataPaths(basePath string) []string {
	candidates := []string{
		ProtectedResourceMetadataRootPath(),
		ProtectedResourceMetadataPath(""),
	}
	if basePath != "" {
		candidates = append(candidates, ProtectedResourceMetadataPath(basePath))
	}
	seen := map[string]bool{}
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if !seen[candidate] {
			seen[candidate] = true
			paths = append(paths, candidate)
		}
	}
	return paths
}

func ProtectedResourceMetadataRootPath() string {
	return "/.well-known/oauth-protected-resource"
}

func ProtectedResourceMetadataPath(basePath string) string {
	return "/.well-known/oauth-protected-resource" + basePath + "/mcp"
}

func ProtectedResourceMetadataURL(req *http.Request, basePath string) string {
	return requestOrigin(req) + ProtectedResourceMetadataPath(basePath)
}

func IssuerURL(req *http.Request, basePath string) string {
	return requestOrigin(req) + basePath
}

func MCPResourceURL(req *http.Request, basePath string) string {
	return IssuerURL(req, basePath) + "/mcp"
}

func absoluteRequestURL(req *http.Request) string {
	return requestOrigin(req) + req.URL.RequestURI()
}

func requestOrigin(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if value := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); value != "" {
		scheme = strings.ToLower(value)
	}
	if req.URL.Scheme != "" {
		scheme = req.URL.Scheme
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	return scheme + "://" + host
}
