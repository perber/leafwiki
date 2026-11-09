# ADR-0024: Security response headers come from one engine middleware, the page policy from the shell

## Status

Proposed (2026-11-09)

## Context

LeafWiki sent no security response headers: no `X-Frame-Options`, no `Referrer-Policy`, no
Content-Security-Policy on the page a browser loads. Any origin could frame the wiki, and the
page had no defense-in-depth policy of its own. The only CSP was the
`sandbox` that `security.SetUntrustedFileHeaders` sets on uploaded files.

Constraints found in the code and the built frontend:

- The built `index.html` has no inline script, style or event handler; the bundle uses no
  `eval`. Mermaid, CodeMirror, Cytoscape and KaTeX insert `<style>` elements and `style=`
  attributes at runtime, and the preview allows `style` and external `http(s)` images and media.
- The PDF embed frames same-origin PDFs from `/assets` and from the revision-asset route,
  which lives under `/api`. Chrome refuses to render a PDF in a sandboxed document.
- Admins inject trusted raw HTML/JS into `<head>` with `InjectCodeInHeader`, by design.
- Go derives the type of the embedded frontend files from the extension; on Windows that reads
  the registry, where a `.js → text/plain` entry is common.
- Some slices build their `/api` groups from `RouterContext.Base` directly, and NoRoute responses
  run only engine-level middleware.
- The Vite dev server serves its own `index.html`; the Go shell is not involved there.

## Decision

**Every response:** `security.Headers` in `internal/http/middleware/security/headers.go`,
registered once with `engine.Use` right after Recovery, sets before the handler runs:

| Header | Value |
|---|---|
| `X-Frame-Options` | `SAMEORIGIN` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `X-Content-Type-Options` | `nosniff`, except below `BasePath + "/static/"` |

Setting them before `c.Next()` lets a handler override them and covers 404s and recovered
panics. The middleware knows nothing about routes or CSP.

**The page:** `shellHandler.serve` in `internal/http/shell.go` sets the shell's
`Content-Security-Policy`, built by `shellCSP` in `internal/http/shell_csp.go`, once
`isShellRequest` has accepted the request and before the responders run:

```
script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'
```

`script-src 'self'` adds defense in depth: no inline script, handler or `javascript:` URL runs on
the page. It is left out while `InjectCodeInHeader` contains active script
(`containsActiveScript`: `<script`, `javascript:` or an inline `on…=` handler, case-insensitive),
and a startup warning says so. A `<meta>` or `<link>` snippet keeps it. The check errs toward
leaving it out: a false positive only costs the backstop for that install, a false negative
breaks the header code visibly and is fixed with the kill switch.

API responses, assets and every request the shell rejects carry no page policy. The knowledge
of which built-in contributor adds script stays next to `headContributor` in `shell.go`; there
is no CSP seam on `ShellParts` or `FrontendConfig`, because there is no second user
([ADR-0007](./0007-extension-points-long-term-goal.md)). The day a second script source
appears, that seam is added first, in its own PR.

**Kill switch:** `--disable-security-headers` / `LEAFWIKI_DISABLE_SECURITY_HEADERS`
(`RouterOptions.DisableSecurityHeaders`) registers no middleware and gives the shell an empty
policy. It is permanent; finer options (an embedding allow-list) would be added next to it.

**Ownership:**

| Header | Owner |
|---|---|
| `X-Frame-Options`, `Referrer-Policy` | `security.Headers` |
| `X-Content-Type-Options` | `security.Headers`; `SetUntrustedFileHeaders` sets the same value on user files (`Set`, idempotent) |
| `Content-Security-Policy` | the shell (document policy), the Markdown responder (`sandbox`, replacing the document policy on a Markdown answer) and `SetUntrustedFileHeaders` (`sandbox` on user files), never the middleware |
| `Content-Disposition` | `SetUntrustedFileHeaders` |

## Alternatives considered

- **One CSP for every response, set in the middleware**: breaks the PDF embed for revision
  assets under `/api` and needs route knowledge in the middleware.
- **Middleware on the `/api` group helpers**: misses the groups built from `RouterContext.Base`
  and every NoRoute response.
- **`default-src`, `style-src`, `img-src`, `font-src`, `media-src`, `connect-src`**: each blocks
  legitimate wiki content (runtime styles, external images) or admin CSS with `@import`, without
  stopping script. `style-src` would need `'unsafe-inline'` and would then be a no-op.
- **A nonce or hash for `InjectCodeInHeader`**: a `<script` string rewrite is unsound, a correct
  one needs an HTML tokenizer (`golang.org/x/net/html` as a direct dependency), and loader
  scripts need `'strict-dynamic'`. An explicit placeholder in the header code would avoid the
  tokenizer and is the follow-up if a nonce mode is wanted.
- **`X-Frame-Options: DENY` / `frame-ancestors 'none'`**: breaks the same-origin PDF embed.
- **HSTS in the app**: sticky per host and the TLS terminator's job; the app cannot tell
  whether the hop to the browser is TLS. `Permissions-Policy` and COOP/COEP/CORP: no
  evidence-based value yet.

## Consequences

- Embedding the wiki in a page on another origin stops working unless the install sets
  `--disable-security-headers` (and, if needed, its own headers in the proxy). Release notes
  must say so.
- `/static` deliberately has no `nosniff`, so a Windows registry entry cannot block the app.
- Installs whose `InjectCodeInHeader` runs script get no script backstop until a nonce mode
  exists.
- Headers are fixed strings; no per-request allocation.
- Revisit when a second script source or a nonce mode is needed (add the seam first), or when
  an embedding allow-list is requested.
