package main

// General API metadata for the experimental OpenAPI (Swagger 2.0) description.
// This file is comment-only: it is read by the swag generator (`make api-docs`)
// and has no runtime code. Operation annotations live next to the handlers.
//
//	@title			LeafWiki API
//	@version		0.x
//	@description	Development API of LeafWiki. This documentation is incomplete, experimental and may change before v1; only a few operations are described so far.
//	@description	API keys (Bearer authentication) are available only when LeafWiki runs with `--enable-api-key-management` and authentication enabled. Existing browser API routes are not newly hidden behind that flag.
//	@description	Browser sessions use the access and refresh cookies (`__Host-leafwiki_at` and `__Host-leafwiki_rt` on secure connections, unprefixed names for allowed insecure development). Session writes are protected by CSRF double-submit: send the `X-CSRF-Token` header matching the CSRF cookie (the `csrf_token` form field is accepted as a fallback).
//	@description	The documented base path is the default mount. A deployment started with `--base-path=/wiki` prepends `/wiki` to every documented URL.
//
//	@license.name	MIT
//	@license.url	https://github.com/perber/leafwiki/blob/main/LICENSE
//
//	@BasePath	/api
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				API key sent as the entire header value `Bearer lw_<prefix>_<secret>`, not just the secret. A request authenticated by a resolved key acts with the effective role of the lesser of the key owner's role and the key's role. A resolved key skips CSRF checks; a header merely being present does not. Routes that reject API-key authentication are browser-session only and say so. Cookies remain an alternative authentication path and are described in prose because Swagger 2.0 cannot express them.
