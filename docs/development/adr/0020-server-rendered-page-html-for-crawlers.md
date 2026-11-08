# ADR-0020: Page HTML for crawlers is rendered in `internal/render` and sanitized by a stricter server policy

## Status

Proposed (2026-10-29)

Open: whether this decision is folded into ADR-0018 (search-engine rendering for public wikis,
proposed in the same series) while that one is still Proposed.

## Context

Search-engine rendering ([#1292](https://github.com/perber/leafwiki/issues/1292)) puts the
rendered page into the SPA shell's `#root` for crawler user agents, through a shell
contributor ([ADR-0017](./0017-spa-shell-contributors.md)). That needs page Markdown rendered
to HTML on the server. Until now only the browser rendered pages (`MarkdownPreview.tsx`:
remark/rehype, then rehype-sanitize with `previewSanitizeSchema.ts`).

Constraints:

- The shell has no Content-Security-Policy. The server-rendered HTML is untrusted page content
  served from the wiki's origin, so its sanitizer is the only XSS barrier.
- The frontend schema allows `id`, `class` and `style` with `clobberPrefix: ''`. In the shell,
  an `id="root"` or `name=` in page content could clobber `#root`, `#mermaid-renderer` or DOM
  globals before the SPA starts; `style` could hide or restyle the shell.
- `src/main.tsx` uses `createRoot`, which replaces `#root`, so the crawler HTML never mixes
  with the SPA's own rendering.
- goldmark and bluemonday are already dependencies (`core/excerpt`, `links`, `search`).
- Wikilinks, folder-relative links and the base path must resolve like in the SPA, without
  the renderer depending on the tree or HTTP.

## Decision

A new top-level package `internal/render` (not `core`: only the `seo` slice uses it and it
owns no state or format) exposes

```go
func RenderHTML(content string, opt Options) (string, error)
type Options struct { BasePath, RoutePath string; ResolveWikiLink func(target string) (routePath string, ok bool) }
```

- **Markdown**: goldmark with tables, strikethrough, task lists and linkify. Raw HTML is passed
  through and then sanitized (kept where the policy allows it, not escaped). No heading IDs,
  no footnotes. Mermaid, math and embeds stay code or text.
- **Wikilinks**: a goldmark inline parser on the shared `markdown.ParseWikiLinks` pattern, so
  wikilinks in code spans and blocks stay literal by construction. The caller's resolver
  decides the target (one page → link; unknown or ambiguous → plain text), so `render`
  imports no tree.
- **Links and images**: an AST transformer resolves Markdown link URLs like the SPA
  (`markdown.ResolveFolderRelative`: relative links point below the page), prefixes the base
  path, escapes route segments and keeps query and fragment. `assets/…`, `/assets/…` and
  `/api/…` images get the base path and stay server-relative. URLs with a scheme are left to
  the sanitizer. Raw HTML URLs are not rewritten.
- **Sanitizer** (`policy.go`, built once): bluemonday with a subset of the GitHub tags the
  frontend schema allows, no `audio`/`video`/`iframe`/`svg`/`math`/`form`/`style`, and **no
  `id`, `name`, `class` or `style` on any element**. `href`/`src` are http, https, mailto or
  same-origin paths; protocol-relative URLs (`//host`, `\\host`, `/\host`, also after leading
  whitespace) are dropped. `input` only as a task-list checkbox. A test pins the frontend
  schema's hash, so changing `previewSanitizeSchema.ts` forces a look at this policy.
- **Containment**: after the sanitizer, the output is parsed as a fragment in the context of a
  `div` and serialized again (`balanceHTML`). The sanitizer drops tags but does not balance
  them, so a stray `</div>` would close `#root` and an unclosed element would swallow the shell.
  It has to run last, because dropping tags is what can leave the input unbalanced.
  The parsed tree is then checked against the same element and attribute tables the
  sanitizer is built from (`policy.go`); anything outside them (another element, a namespace,
  a comment, a foreign attribute) is an error, so a later policy mistake means no rendered
  body instead of XSS.
- **Limits**: any client can claim to be a crawler, so one render is bounded. A linear
  pre-pass with goldmark's block parser (no inline parsers, no table extension) gives the
  inline containers without block quote or list prefixes; on it the renderer refuses more
  than 60,000 blocks, nesting deeper than 64, more than 50,000 table cells (goldmark pads
  every row to the header's width, so a 14 KB table can render 40 MB; counted from the first
  delimiter row with the widest line), more than 64 KB or more than 400 ``[ < * _ ~ ` `` in one
  paragraph, heading or table row (goldmark matches them in quadratic time per container), and
  more than 15,000 `<` on the page, in raw HTML blocks (closing line included) and inline
  (x/net/html appends text separated by ignored end tags such as `x</td>x</td>` to one growing
  node, also across table rows). Before it: at most 256 KB of Markdown, 20,000 `*`/`_`, 64
  nesting markers per line and 5,000 link reference definitions without a blank line between them (quadratic in
  goldmark's definition parser, already in the pre-pass). After it: at most 4 MB of HTML before
  sanitizing. Past a limit `RenderHTML` returns `ErrTooComplex`; the
  caller then serves the plain shell, never a 500, and logs no page content. Caching per page
  version and bounding concurrent renders belong to the PR that wires the renderer in.

## Alternatives considered

- **Reuse the frontend schema 1:1**: it allows `id`/`class`/`style` with no clobber prefix,
  which is acceptable inside the SPA's rendered preview but not in a document the SPA has not
  booted in yet.
- **Escape all raw HTML**: safer by a margin, but wikis use `<details>`, `<kbd>` and tables in
  raw HTML; crawlers would see tag soup that differs from what visitors see.
- **Run the frontend pipeline on the server** (Node, or a JS engine in Go): a second runtime
  in a single-binary product that also runs on a Raspberry Pi.
- **Renderer inside the `seo` slice**: works today, but rendering is not HTTP-shaped and a
  later feature (exports, link previews) would have to import a slice.
- **Keep `//host` links**: they are external links like `https://host`, but they are also the
  classic open-redirect/foreign-origin shape and nothing in normal Markdown needs them.

## Consequences

- Crawlers see the page's content, links and images; the visible page in a browser is still
  the SPA's rendering. The two renderers can diverge in details (flexible markers, image size
  syntax, footnotes, math), which only affects what crawlers index.
- The cost is about 1.5 ms per KB of dense Markdown on a desktop CPU (`BenchmarkRenderHTML`,
  100 KB), more on a Raspberry Pi; it runs per crawler request only.
- Every new tag or attribute the crawler HTML needs is a policy change with an XSS-corpus
  test (`policy_test.go`). Revisit when the shell gets a CSP, or when the frontend schema
  changes in a way the pinned hash flags.
