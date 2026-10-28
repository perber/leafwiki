// Package seo makes public wikis readable for search engines (#1292,
// ADR-0018). It plugs into the SPA shell seam of internal/http (ADR-0017)
// and is active only while public access is on: every registration goes
// through PublicOnly, which declines without reading anything otherwise.
//
// Canonical URLs come from --public-url only, never from request headers.
// Requests reaching this package are always anonymous; it reads pages through
// narrow reader interfaces that the composition root implements with the
// pages slice's read use cases.
package seo
