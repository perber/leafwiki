package oauth

import httpinternal "github.com/perber/wiki/internal/http"

// Routes registers the OAuth discovery, authorization, and token endpoints.
type Routes struct {
	service *Service
}

func NewRoutes(service *Service) *Routes {
	return &Routes{service: service}
}

func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	if r.service == nil || !ctx.Opts.MCPEnabled || ctx.Opts.AuthDisabled || !httpinternal.IsLoopbackHost(ctx.Opts.MCPBindHost) {
		return
	}

	for _, path := range AuthorizationServerMetadataPaths(ctx.Opts.BasePath) {
		ctx.Engine.GET(path, r.handleAuthorizationServerMetadata(ctx))
	}
	for _, path := range ProtectedResourceMetadataPaths(ctx.Opts.BasePath) {
		ctx.Engine.GET(path, r.handleProtectedResourceMetadata(ctx))
		ctx.Engine.OPTIONS(path, r.handleProtectedResourceMetadata(ctx))
	}

	ctx.Base.GET("/oauth/authorize", r.handleAuthorize(ctx))
	ctx.Base.POST("/oauth/authorize", r.handleAuthorize(ctx))
	ctx.Base.GET("/oauth/approval", r.handleApprovalDetails(ctx))
	ctx.Base.POST("/oauth/register", r.handleRegister)
	ctx.Base.POST("/oauth/token", r.handleToken)
}

var _ httpinternal.RouteRegistrar = (*Routes)(nil)
