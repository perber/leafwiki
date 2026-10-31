# ADR-0018: Public wikis are rendered for search engines through the `seo` slice, gated on public access

## Status

Proposed (2026-10-28)

## Context

A public LeafWiki is invisible to most search engines and link previews: every URL answers
with the same empty SPA shell, `/` has no content of its own, and there is no `robots.txt`,
sitemap or canonical link ([#1292](https://github.com/perber/leafwiki/issues/1292)). The work
is split into a series of PRs; this ADR fixes the rules they share.

Constraints:

- [ADR-0017](./0017-spa-shell-contributors.md) gives the seam: shell responders and
  contributors registered on `FrontendConfig`. Shell requests are always anonymous, so
  anything they show must be publicly readable.
- Public access ("public mode") is a runtime setting: `internal/publicaccess.Service` is
  either pinned by `--public-access` / `--disable-auth` or toggled by an admin in Settings
  without a restart. The router and the auth middleware read it per request
  (`publicaccess.Provider.Enabled()`).
- The page-view use cases in `internal/wiki/pages` (`get_page_view.go`) are read helpers, not
  a visibility layer. Today a public wiki shows every page to anonymous readers, so that is
  enough; partial visibility will go through `internal/scopedread`
  ([ADR-0015](./0015-scoped-read-boundary.md)).
- LeafWiki usually runs behind a reverse proxy. `Host`, `X-Forwarded-Host` and
  `X-Forwarded-Proto` are client-controlled unless the proxy rewrites them, so building
  absolute URLs from them allows cache poisoning and links to foreign hosts.
- Slices must not import each other (`internal/architecture_test.go`).

## Decision

**Scope and switch.** SEO is active exactly when public access is on, read per request. There
is no separate SEO flag. A private wiki behaves as before: nothing is redirected, rendered or
listed, and no page is read for it.

**Where it lives.** A new slice `internal/wiki/seo` holds everything SEO-specific. It
registers only through existing seams (shell responders and contributors on `FrontendConfig`,
later a `RouteRegistrar` for its own routes); shared code gets no `if seo` branches.

- `seo.PublicOnly` wraps **every** registration of the slice. While public access is off it
  declines before the wrapped responder or contributor runs, so it reads nothing. The
  composition root passes it `WikiOptions.PublicAccess` (the same `publicaccess.Service` the
  router gets).
- Reads go through narrow reader interfaces with result types owned by `seo` (for example
  `seo.FirstPageReader` returning `*seo.FirstPage`), one per use case. The composition root
  implements them in `internal/wiki/seo_adapter.go` with the pages slice's read use cases, like
  `import_adapter.go` does for the importer. `seo` never imports `pages` or `core/tree`.
- Use cases have the usual `Execute(ctx, in XxxInput)` shape.

**Absolute URLs.** `--public-url` (a server flag since it moved out of the SMTP flags, see
[ADR-0011](./0011-smtp-cli-env-optional-feature.md)) is the only source for canonical links,
sitemap entries and any other absolute URL. Request headers are never used for it. Without
`--public-url`, features that need an absolute URL leave it out instead of guessing.
Redirects need no absolute URL and use a server-relative path.

**Root redirect** (this ADR's first user). In a public wiki, `GET /` (with `--base-path`:
`/<base>` and `/<base>/`) answers `302 Found` with `Location: <base path>/<first page route>`.
The first page is the first top-level page or section in navigation order
(`TreeService.FirstRootChildPath` via `pages.GetFirstPagePathUseCase`), the same target the
SPA's `RootRedirect` picks. The base path and every route segment are percent-escaped, so the
location never contains CR, LF, `?`, `#` or `\`, and one starting with `//` is refused. An
empty tree or a tree that is not loaded yet means no redirect: the shell is served as before.
It is a 302, not a 301, because the first page changes when pages are sorted, renamed or
public access is turned off.

**How the next PRs dock on:**

- **Meta tags and canonical link** (3c): a contributor behind `PublicOnly` adds title,
  description, Open Graph tags and `<link rel="canonical">` built from `--public-url`; that PR
  also validates `--public-url` without SMTP.
- **`robots.txt` and `sitemap.xml`** (4): routes of the `seo` slice. The handlers check the
  same public-access gate; a private wiki keeps today's responses for these paths. Sitemap URLs
  use `--public-url`; without it there is no sitemap.
- **Markdown representation** (5): a responder behind `PublicOnly` answers
  `Accept: text/markdown` with the page's Markdown. It adds `Vary: Accept` whether it handles
  the request or not.
- **Rendered body for search engines** (6): a contributor behind `PublicOnly` puts the
  rendered page into `#root` for crawler user agents, sets `Vary: User-Agent` through
  `ShellParts` and makes the shell `Cache-Control: no-cache`, as ADR-0017 requires.

HEAD requests for SPA routes are outside this decision (the shell does not answer HEAD today);
fixing that is a separate change.

## Alternatives considered

- **A separate `--enable-seo` flag**: a second switch that can disagree with public access
  (SEO on a private wiki would leak nothing, but would be confusing; SEO off on a public wiki
  is what `robots.txt` is for). Public access already says "anyone may read this".
- **Absolute URLs from `Host` / `X-Forwarded-*`**: needs a trusted-proxy configuration to be
  safe and still breaks behind proxies that don't set them; one explicit flag is simpler to
  reason about.
- **Server-side redirect of `/` in every wiki**: a private wiki redirects in the SPA after
  login already; doing it on the server would read the tree for anonymous requests that may
  not see it.
- **`seo` importing `pages` directly**: breaks the slice boundary; the adapter keeps `seo`
  testable with fakes and lets the read side move to `internal/scopedread` later without
  touching `seo`.
- **301 for the root redirect**: browsers and crawlers cache it indefinitely, so a re-sorted
  tree or a wiki switched back to private would keep sending visitors to the old page.

## Consequences

- In public wikis `/` answers 302 instead of the shell; crawlers land on a page with content.
  Clients that fetched `/` and expected HTML now have to follow the redirect.
- Switching public access off in Settings turns all SEO behaviour off on the next request,
  with no restart.
- Every SEO feature adds one registration in `seoShellResponders` (or a contributor
  counterpart) plus a reader method on the adapter; the router and the pages slice stay
  unchanged.
- Once anonymous readers may see only part of a wiki (drafts, public sections), the adapters
  switch to `internal/scopedread`; revisit this ADR then, because "public access on" will no
  longer mean "everything is public".
