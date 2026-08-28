package main

import "github.com/urfave/cli/v3"

// proxyOptions holds the options for reverse-proxy authentication and external URLs.
type proxyOptions struct {
	enableHTTPRemoteUser           bool
	httpRemoteUserHeader           string
	enableHTTPRemoteUserAutoCreate bool
	httpRemoteUserEmailHeader      string
	httpRemoteUserDefaultRole      string
	trustedProxyIPs                string
	httpRemoteUserLogoutURL        string
	loginURL                       string
	logoutURL                      string
	userManagementURL              string
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *proxyOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:        "enable-http-remote-user",
			Destination: &o.enableHTTPRemoteUser,
			Category:    catProxyAuth,
			Usage:       "enable reverse-proxy authentication via HTTP header",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_HTTP_REMOTE_USER"),
		},
		&cli.StringFlag{
			Name:        "http-remote-user-header-name",
			Destination: &o.httpRemoteUserHeader,
			Category:    catProxyAuth,
			Usage:       "HTTP header carrying the username or email from a trusted proxy",
			Value:       "Remote-User",
			Sources:     envVars("LEAFWIKI_HTTP_REMOTE_USER_HEADER_NAME"),
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "enable-http-remote-user-auto-create",
			Destination: &o.enableHTTPRemoteUserAutoCreate,
			Category:    catProxyAuth,
			Usage:       "auto-provision users asserted by the proxy but unknown to LeafWiki",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_HTTP_REMOTE_USER_AUTO_CREATE"),
		},
		&cli.StringFlag{
			Name:        "http-remote-user-email-header-name",
			Destination: &o.httpRemoteUserEmailHeader,
			Category:    catProxyAuth,
			Usage:       "HTTP header carrying the email for auto-created users",
			Sources:     envVars("LEAFWIKI_HTTP_REMOTE_USER_EMAIL_HEADER_NAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "http-remote-user-default-role",
			Destination: &o.httpRemoteUserDefaultRole,
			Category:    catProxyAuth,
			Usage:       "role assigned to auto-created users; must not be admin",
			Value:       "viewer",
			Sources:     envVars("LEAFWIKI_HTTP_REMOTE_USER_DEFAULT_ROLE"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "trusted-proxy-ips",
			Destination: &o.trustedProxyIPs,
			Category:    catProxyAuth,
			Usage:       "comma-separated trusted proxy IPs/CIDRs (e.g. 127.0.0.1,172.18.0.0/16)",
			Sources:     envVars("LEAFWIKI_TRUSTED_PROXY_IPS"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "http-remote-user-logout-url",
			Destination: &o.httpRemoteUserLogoutURL,
			Category:    catProxyAuth,
			Usage:       "deprecated: use --logout-url instead",
			Sources:     envVars("LEAFWIKI_HTTP_REMOTE_USER_LOGOUT_URL"),
			Validator:   redirectURLValidator("http-remote-user-logout-url"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "login-url",
			Destination: &o.loginURL,
			Category:    catURLs,
			Usage:       "URL the frontend redirects to instead of the built-in login form (e.g. external SSO/IdP)",
			Sources:     envVars("LEAFWIKI_LOGIN_URL"),
			Validator:   redirectURLValidator("login-url"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "logout-url",
			Destination: &o.logoutURL,
			Category:    catURLs,
			Usage:       "URL the frontend redirects to after logout (e.g. external SSO/IdP)",
			Sources:     envVars("LEAFWIKI_LOGOUT_URL"),
			Validator:   redirectURLValidator("logout-url"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "user-management-url",
			Destination: &o.userManagementURL,
			Category:    catURLs,
			Usage:       "external user-management page; replaces the built-in User Management UI with a link",
			Sources:     envVars("LEAFWIKI_USER_MANAGEMENT_URL"),
			Validator:   redirectURLValidator("user-management-url"),
			Config:      trimmed,
		},
	}
}
