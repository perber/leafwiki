# ADR-0018: Public wikis are rendered for search engines through the `seo` slice, gated on public access

## Status

Accepted (2026-11-08; proposed 2026-10-28).

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

## Later changes

- **2026-10 (#1292, meta tags, PR 3c):** details fixed while implementing the meta tags:
  - `<title>` is `<page title> - <site name>` (branding site name, `LeafWiki` when unset), the
    format the SPA sets once it has loaded (`seo.DocumentTitle` and `documentTitle.ts` point at
    each other). `og:type` is `website`.
  - The description is the page property with the exact key `description`, whitespace
    collapsed and capped at 360 runes (twice the excerpt length, cut the same way); otherwise an
    excerpt of the first 2 KB of the body. A multi-line
    value (`description: |`) is not a page property (the pages slice drops values with line
    breaks), so such pages get the excerpt.
  - The canonical link and `og:url` are the scheme and host of `--public-url` plus the base path
    and the escaped route path. The path of `--public-url` is only checked against
    `--base-path` at startup (warning on mismatch), because email links use `--public-url` as
    given.
  - `--public-url` is checked without SMTP: unset in a public wiki logs a hint; an invalid value
    is ignored with a warning, and stays fatal with `--smtp-host`.
  - Shells with page meta are served with `Cache-Control: no-cache` (`ShellParts.NoCache`,
    ADR-0017). Non-page routes, unknown pages and a tree that is not loaded get the plain shell.
- **2026-11 (#1292, HEAD):** the shell now answers HEAD like GET without a body (see
  [ADR-0017](./0017-spa-shell-contributors.md), "Later changes"), so the root redirect and the
  page meta apply to HEAD too. The sentence above that HEAD is outside this decision is
  superseded.
- **2026-11 (#1292, `robots.txt` and `sitemap.xml`):** the slice's first own routes,
  registered through `seo.Routes` (a `RouteRegistrar`) for GET and HEAD; both handlers go
  through `PublicOnly.Handler(on, off)`. Amends the sentence above that a private wiki keeps
  today's responses for these paths (that was the SPA shell with status 200):
  - Private wiki: `robots.txt` is `User-agent: *` / `Disallow: /`, `sitemap.xml` is 404.
    Neither reads a page.
  - Public wiki: `robots.txt` is `User-agent: *` / `Allow: /`, plus `Sitemap: <url>` only with
    `--public-url`. `/api` is never disallowed, because search engines render the SPA, which
    reads pages through it.
  - `sitemap.xml` lists every page and section in navigation order with `<lastmod>` (RFC 3339,
    UTC; left out when unknown), from one locked tree read (`TreeService.PageRoutes`, no file
    reads). The root is not listed: it is a 302 to the first page, which is listed itself.
    Without `--public-url` there is no sitemap (404). Above 50,000 routes (the sitemaps.org
    limit per file) only the first 50,000 are listed and a warning is logged once per process;
    a sitemap index is left for when a wiki needs it.
  - Both are `Cache-Control: no-cache`, so toggling public access is seen on the next fetch.
  - Under `--base-path` both are served below the base path only. Crawlers read `robots.txt`
    only at the host root, so there it has no effect; the sitemap still works (its scope is its
    directory), but has to be submitted to the search engine by hand.
  - The document is streamed with `encoding/xml`, never held whole in memory.
    `BenchmarkSitemap` (`internal/wiki/seo`, fake lister, so without the tree walk) allocates
    2.5 MB / 70,000 times for 10,000 routes and 13 MB / 350,000 times for 50,000 (seven per
    entry, mostly the URL strings and the encoder's reflection). Time was 0.27 s and 0.83 s
    on a heavily loaded laptop, so treat it as an upper bound; rerun it on a Raspberry Pi
    before raising the limit or adding fields per entry.
- **2026-11 (#1292, Markdown representation):** details fixed while implementing it:
  - A page route is answered with the page's Markdown (its content without frontmatter, as
    the page-view read returns it) when the `Accept` header lists `text/markdown` explicitly
    with `q > 0` and gives `text/html` no higher `q`. `text/html` takes the `q` of its most
    specific match (`text/html`, then `text/*`, then `*/*`); `*/*` or `text/*` alone never
    select Markdown. A tie goes to Markdown. Ranges with a malformed `q` (outside the RFC 9110
    qvalue grammar) are ignored; several `Accept` lines are read as one list.
  - The response is `200` with `Content-Type: text/markdown; charset=utf-8`,
    `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox` (the body is raw user
    content; no `Content-Disposition`, it stays readable inline), `Cache-Control: no-cache` and
    `Content-Length`; HEAD gets the same headers without a body.
  - Quoted parameter values in `Accept` are not parsed: a quoted `,` splits the list there. A
    client can only pick a representation it could ask for directly that way.
  - Unknown pages, non-page routes (including `/p/:id` permalinks) and a tree that is not
    loaded decline, so the shell is served. `/` stays the root redirect, which is registered
    before the Markdown responder.
  - `Vary: Accept` is added (not set, and not twice when a `Vary` line already lists
    `Accept` or `*`) on every shell request of a public wiki, handled or declined. While
    public access is off `PublicOnly` declines first, so a private wiki's shell carries no
    `Vary: Accept`: it is the same for every `Accept` value, and the `no-cache` on the
    Markdown variant keeps an on→off toggle from serving a cached one.
  - Reader interfaces are one per read; use cases that need the same read share its interface
    (page meta and Markdown share `seo.PageReader`, which lives in `page.go`).
- **2026-11 (#1292, permalinks):** a permalink `/p/<id>` or `/p/<id>/<slug>` gets the page
  meta of the page the ID points at, with that page's route path as canonical link and
  `og:url`; the slug is decorative and may be stale. The shell is served (200), not a server
  redirect: the SPA already replaces the URL with the page's path, and the canonical link tells
  search engines which URL to index. `/p/:id/:slug?` is the only SPA route the `seo` slice
  knows (`permalinkID` in `urls.go`, shared by the page meta and the Markdown responder,
  mirroring `router.tsx`). Like react-router, it matches the `p` segment in any case
  (`/P/<id>` is a permalink too). Anything else under `/p/` (`/p/` alone, an empty ID, more
  segments) is an ordinary page route there and here. An encoded `%2F` in the ID is decoded by
  the server, so `/p/<id>%2Fx` is served as `<id>` with the slug `x`, while the SPA keeps it
  inside the ID; that is harmless, because the canonical link still names a real page. The ID
  is resolved through the pages slice's permalink lookup behind `seo.PermalinkResolver`; an
  unknown ID or a tree that is not loaded means no meta. Permalinks have no Markdown
  representation: the Markdown responder declines them without reading, so they get the shell.
