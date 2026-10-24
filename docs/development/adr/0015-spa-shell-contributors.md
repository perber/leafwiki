# ADR-0015: The SPA shell is rendered through registered responders and contributors

## Status

Proposed (2026-10-19)

## Context

Every GET that no route matches (deep links like `/docs/setup`) is answered with the built
`index.html` of the SPA. Until now `internal/http/router.go` filled that document inline: site
name, base path, favicon, the `./static/` rewrite, the custom stylesheet tag and the admin's
`InjectCodeInHeader`. Search engine rendering
([#1292](https://github.com/perber/leafwiki/issues/1292)) needs more: a page title, meta and
Open Graph tags, a canonical link, readable page content inside `#root`, and later other
representations of the same URL (redirects, Markdown). Adding each of these to the router would
make it learn about pages, visibility and SEO, which [rule 1](../rules.md) and
[ADR-0007](./0007-extension-points-long-term-goal.md) rule out.

Constraints:

- The fallback route is `engine.NoRoute`. It runs only engine-level middleware (request log,
  metrics, recovery), not the base group's remote-user or API-key authentication, so the
  router never knows who is asking.
- The built `index.html` is not in git and not built in CI; tests of the seam can't depend on it.
- The shell is served on every SPA navigation that reloads the page, including on a Raspberry Pi.

## Decision

`internal/http/shell.go` defines two seams, registered on `FrontendConfig` and filled in
`(*Wiki).FrontendConfig()` in `internal/wiki/wiki_routes.go` (the composition root, like the
other runtime data the SPA fallback needs):

```go
type ShellRequest struct { Request *http.Request; RoutePath string; BasePath string }
type ShellParts struct { Title string; Head template.HTML; Body template.HTML }
type ShellResponder interface { RespondShell(w http.ResponseWriter, req ShellRequest) (handled bool) }
type ShellContributor interface { ContributeShell(ctx context.Context, req ShellRequest) ShellParts }
```

Request flow on the fallback route:

1. Strip `BasePath` (`RoutePath` always starts with `/`). Only GET requests outside `/api`,
   `/assets`, `/static` and `/branding` count as shell requests; everything else keeps today's
   404.
2. Responders run in registration order; the first returning `handled=true` has written the
   response and ends the request. They use the stdlib `http.ResponseWriter` (`http.Redirect`
   etc.); the router passes gin's writer so logging and metrics see the status. A declining
   responder may set headers only, never the status or a body. Responders run
   even when the frontend is not embedded (dev builds); only the shell itself needs `dist/`.
3. Contributors run in registration order. The first non-empty `Title` wins, otherwise the
   site name. `Head` and `Body` are concatenated in order. The custom stylesheet tag and
   `InjectCodeInHeader` are built-in contributors appended after all registered ones, so admin
   code always comes last.
4. One `strings.NewReplacer` pass fills `{{__PAGE_TITLE__}}`, `{{__SITE_NAME__}}`,
   `{{__BASE_PATH__}}`, `{{__FAVICON_HREF__}}`, `{{__BODY__}}` (inside `#root` in
   `ui/leafwiki-ui/index.html`) and rewrites `"./static/`. Contributed text is never scanned for
   placeholders. All head parts are then injected before `</head>` in one step, so a literal
   `</head>` inside one part can't capture the parts after it.

Escaping: the router escapes `Title` and its own values. `Head` and `Body` are `template.HTML`,
so the type makes the trust explicit: the contributor escapes. `MetaTag`, `MetaProperty` and
`LinkTag` build escaped tags so contributors don't hand-escape attributes.

Contract for responders and contributors:

- **Requests are always anonymous.** Remote-user and API-key authentication are never resolved
  on this route. Expose only publicly readable content and decline cheaply when public access is
  off. Read pages through the same read use cases as the API, never from `core/tree` directly.
  Those use cases are not yet a visibility layer for every reader; once drafts or per-page
  visibility exist, shell features read through that layer like everyone else.
- They run on every SPA GET; keep them cheap.
- Never panic; handle and log own errors and return empty parts or `false`.
- A responder whose decision depends on a request header (e.g. `Accept`) adds the `Vary` header
  itself, also when it declines. A contributor has no writer; the first contributor whose parts
  depend on a header (e.g. `User-Agent`) adds a `Vary` field to `ShellParts`, and the shell gets
  `Cache-Control: no-cache` once it carries page content.
- `RoutePath` is decoded but not cleaned; resolve it through the route lookup, never as a file
  path.
- Registration order is the composition root's responsibility.
- A non-200 shell (404/410 for missing pages) will be a field on `ShellParts` when needed, not a
  responder that re-renders the shell.

## Alternatives considered

- **Keep extending the router inline**: every SEO feature would add page and visibility
  knowledge to shared code; rejected by rule 1.
- **`*gin.Context` in the responder signature**: ties features to gin and lets them reach
  request state the contract says they don't have; the stdlib writer is enough for redirects
  and alternative representations.
- **`html/template` for the index**: the built `index.html` is not authored as a Go template
  and Vite owns its content; placeholder replacement keeps the build untouched, and one replacer
  pass keeps contributed text out of placeholder expansion.
- **Registration through `RouteRegistrar`/`RouterContext`**: registrars add routes to the
  base group; the shell is the engine-level fallback with different (anonymous) semantics.
  Keeping both lists on `FrontendConfig` makes that difference visible at the registration
  site.
- **A head placeholder in `index.html`**: injecting before `</head>` already worked for the
  stylesheet and admin code; a second mechanism for the same thing would be redundant.

## Consequences

- Features add titles, meta tags, server-rendered content or alternative responses by
  registering in the composition root; the router stays unchanged.
- The anonymous-request rule means per-user content can never be served through the shell.
  Revisit if the fallback route ever runs inside the authenticated base group.
- `{{__BODY__}}` is a new placeholder inside `#root`; the Vite dev server strips it
  (`vite.config.ts`) because it does not run the Go rendering.
- Rendering costs more allocations per request: `BenchmarkFrontendHTMLTemplate` went from 27 to
  48 allocations (about 0.5 KB more), mostly for building the replacer per request. Speed was
  about the same. If contributors make this matter on small hardware, split the index at the
  fixed placeholders once at startup; measure with the benchmark when adding contributors.
- A placeholder inside the site name is no longer expanded (it was, by the next
  `strings.ReplaceAll`). This is the only output change for existing configurations.
- The custom stylesheet tag still inserts the base path unescaped; this predates the seam and is
  tracked separately.
