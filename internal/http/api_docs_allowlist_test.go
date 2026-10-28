package http_test

// undocumentedAPIRouteLiteral lists, one exact operation per entry, the real
// API operations that docs/api/swagger.json does not describe yet. It is
// documentation debt, not an authorization allowlist.
//
// Keys are "METHOD /api/path" with Swagger {param} segments; values are
// "<package>: <reason>". Constant string keys make the compiler reject
// duplicates. There is no wildcard or prefix matching: a new route needs real
// documentation or its own reviewed entry, and a package PR that documents
// its operations removes their entries.
var undocumentedAPIRouteLiteral = map[string]string{
	// auth (internal/wiki/auth)
	"POST /api/auth/invite/accept":          "auth: pending package documentation",
	"POST /api/auth/login":                  "auth: pending package documentation",
	"POST /api/auth/login/totp":             "auth: pending package documentation",
	"POST /api/auth/logout":                 "auth: pending package documentation",
	"GET /api/auth/me":                      "auth: pending package documentation",
	"POST /api/auth/password/forgot":        "auth: pending package documentation",
	"POST /api/auth/password/reset":         "auth: pending package documentation",
	"POST /api/auth/refresh-token":          "auth: pending package documentation",
	"GET /api/config":                       "auth: pending package documentation",
	"GET /api/users":                        "auth: pending package documentation",
	"POST /api/users":                       "auth: pending package documentation",
	"POST /api/users/invite":                "auth: pending package documentation",
	"PUT /api/users/me/password":            "auth: pending package documentation",
	"POST /api/users/me/totp/disable":       "auth: pending package documentation",
	"POST /api/users/me/totp/setup/confirm": "auth: pending package documentation",
	"POST /api/users/me/totp/setup/start":   "auth: pending package documentation",
	"GET /api/users/me/totp/status":         "auth: pending package documentation",
	"PUT /api/users/{id}":                   "auth: pending package documentation",
	"DELETE /api/users/{id}":                "auth: pending package documentation",
	"POST /api/users/{id}/invite/resend":    "auth: pending package documentation",

	// pages (internal/wiki/pages)
	"GET /api/favorites":                    "pages: pending package documentation",
	"POST /api/pages":                       "pages: pending package documentation",
	"GET /api/pages/by-path":                "pages: pending package documentation",
	"GET /api/pages/by-title":               "pages: pending package documentation",
	"POST /api/pages/convert/{id}":          "pages: pending package documentation",
	"POST /api/pages/copy/{id}":             "pages: pending package documentation",
	"POST /api/pages/ensure":                "pages: pending package documentation",
	"GET /api/pages/lookup":                 "pages: pending package documentation",
	"GET /api/pages/permalink/{id}":         "pages: pending package documentation",
	"GET /api/pages/slug-suggestion":        "pages: pending package documentation",
	"GET /api/pages/{id}":                   "pages: pending package documentation",
	"PUT /api/pages/{id}":                   "pages: pending package documentation",
	"DELETE /api/pages/{id}":                "pages: pending package documentation",
	"PUT /api/pages/{id}/favorite":          "pages: pending package documentation",
	"DELETE /api/pages/{id}/favorite":       "pages: pending package documentation",
	"PUT /api/pages/{id}/move":              "pages: pending package documentation",
	"PUT /api/pages/{id}/pin":               "pages: pending package documentation",
	"POST /api/pages/{id}/refactor/apply":   "pages: pending package documentation",
	"POST /api/pages/{id}/refactor/preview": "pages: pending package documentation",
	"PUT /api/pages/{id}/sort":              "pages: pending package documentation",
	"GET /api/tree":                         "pages: pending package documentation",

	// assets (internal/wiki/assets)
	"GET /api/pages/{id}/assets":           "assets: pending package documentation",
	"POST /api/pages/{id}/assets":          "assets: pending package documentation",
	"PUT /api/pages/{id}/assets/rename":    "assets: pending package documentation",
	"DELETE /api/pages/{id}/assets/{name}": "assets: pending package documentation",

	// revisions (internal/wiki/revisions)
	"GET /api/pages/{id}/revisions":                            "revisions: pending package documentation",
	"GET /api/pages/{id}/revisions/compare":                    "revisions: pending package documentation",
	"GET /api/pages/{id}/revisions/latest":                     "revisions: pending package documentation",
	"GET /api/pages/{id}/revisions/{revisionId}":               "revisions: pending package documentation",
	"GET /api/pages/{id}/revisions/{revisionId}/assets/{name}": "revisions: pending package documentation",
	"POST /api/pages/{id}/revisions/{revisionId}/restore":      "revisions: pending package documentation",

	// search (internal/wiki/search)
	"GET /api/search":        "search: pending package documentation",
	"GET /api/search/status": "search: pending package documentation",

	// links (internal/wiki/links)
	"GET /api/links/broken":     "links: pending package documentation",
	"GET /api/pages/{id}/links": "links: pending package documentation",

	// branding (internal/wiki/branding)
	"GET /api/branding":            "branding: pending package documentation",
	"PUT /api/branding":            "branding: pending package documentation",
	"POST /api/branding/favicon":   "branding: pending package documentation",
	"DELETE /api/branding/favicon": "branding: pending package documentation",
	"POST /api/branding/logo":      "branding: pending package documentation",
	"DELETE /api/branding/logo":    "branding: pending package documentation",

	// avatar (internal/wiki/avatar)
	"POST /api/user/avatar":   "avatar: pending package documentation",
	"DELETE /api/user/avatar": "avatar: pending package documentation",

	// usersettings (internal/wiki/usersettings)
	"GET /api/user-settings": "usersettings: pending package documentation",
	"PUT /api/user-settings": "usersettings: pending package documentation",

	// apikeys (internal/wiki/apikeys)
	"GET /api/api-keys":         "apikeys: pending package documentation",
	"POST /api/api-keys":        "apikeys: pending package documentation",
	"DELETE /api/api-keys/{id}": "apikeys: pending package documentation",

	// importer (internal/wiki/importer)
	"POST /api/import/execute": "importer: pending package documentation",
	"GET /api/import/plan":     "importer: pending package documentation",
	"POST /api/import/plan":    "importer: pending package documentation",
	"DELETE /api/import/plan":  "importer: pending package documentation",

	// resync (internal/wiki/resync)
	"POST /api/admin/resync":       "resync: pending package documentation",
	"GET /api/admin/resync/status": "resync: pending package documentation",

	// backup (internal/wiki/backup)
	"GET /api/admin/backup/config":       "backup: pending package documentation",
	"POST /api/admin/backup/config":      "backup: pending package documentation",
	"POST /api/admin/backup/config/test": "backup: pending package documentation",
	"POST /api/admin/backup/disable":     "backup: pending package documentation",
	"POST /api/admin/backup/force-push":  "backup: pending package documentation",
	"POST /api/admin/backup/pull":        "backup: pending package documentation",
	"POST /api/admin/backup/push":        "backup: pending package documentation",
	"GET /api/admin/backup/status":       "backup: pending package documentation",
	"GET /api/backup/alert":              "backup: pending package documentation",

	// snapshot (internal/wiki/snapshot)
	"GET /api/admin/snapshot":               "snapshot: pending package documentation",
	"POST /api/admin/snapshot":              "snapshot: pending package documentation",
	"GET /api/admin/snapshot/status":        "snapshot: pending package documentation",
	"DELETE /api/admin/snapshot/{id}":       "snapshot: pending package documentation",
	"GET /api/admin/snapshot/{id}/download": "snapshot: pending package documentation",

	// restore (internal/wiki/restore)
	"POST /api/admin/restore/self-restart": "restore: pending package documentation",
	"GET /api/admin/restore/status":        "restore: pending package documentation",
	"POST /api/admin/restore/upload":       "restore: pending package documentation",
	"POST /api/admin/restore/{id}":         "restore: pending package documentation",

	// instancesettings (internal/wiki/instancesettings)
	"PUT /api/admin/settings/public-access": "instancesettings: pending package documentation",
	"PUT /api/admin/settings/toc-display":   "instancesettings: pending package documentation",
}

var undocumentedAPIRoutes, undocumentedAPIRoutesErr = parseAPIRouteExceptions(undocumentedAPIRouteLiteral)
