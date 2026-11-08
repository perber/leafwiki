// Package seo makes public wikis readable for search engines (#1292,
// ADR-0018). It plugs into the SPA shell seam of internal/http (ADR-0017)
// and is active only while public access is on: every registration goes
// through PublicOnly, which declines without reading anything otherwise.
//
// Canonical URLs come from --public-url only, never from request headers.
// Requests reaching this package are always anonymous; it reads pages through
// narrow reader interfaces that the composition root implements with the
// pages slice's read use cases.
//
// File shape: one file per feature (root_target.go, page_meta.go, ...) holds
// the feature's seo-owned result types, its reader interface and its use
// case; a reader shared by several use cases lives in its own file (page.go);
// shell.go holds PublicOnly and every shell responder and contributor;
// routes.go holds the slice's own routes; urls.go holds the URL helpers the
// features share. Tests mirror the files.
package seo
