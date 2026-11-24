# ADR-0025: Literal search words also match inside longer words, through a trigram table, ranked after token matches

## Status

Proposed (2026-11-24). The table and its write paths ship first; the query side
(`substringOnlyWhere`, ranking, highlight) follows in the next pull request.

## Context

The `pages` FTS5 table in `internal/search` uses `unicode61` with the token characters
`-_/+#.`, so `Markdown-Syntax`, `ingress-nginx`, `backup_daily.sh` and `2026-08-26` are each one
token. A literal search word becomes a prefix query (`"syntax"*`), and FTS5 can only match a
token from its start. So `syntax` did not find `Markdown-Syntax`, `08-26` did not find a
`## 2026-08-26` heading, and German compounds (`suche` in `Wikisuche`, `beleuchtung` in
`Straßenbeleuchtung`) were not found at all
([discussion #1826](https://github.com/perber/leafwiki/discussions/1826),
[discussion #1472](https://github.com/perber/leafwiki/discussions/1472)). Changing the token
characters cannot fix compounds, and it would change every existing query.

The bundled `modernc.org/sqlite` (SQLite 3.54.1) ships FTS5's `trigram` tokenizer with
`remove_diacritics` and contentless tables with `contentless_delete`.

## Decision

- `searchSchema` adds `pages_trigram(title, headings, content)`, `content=''`,
  `contentless_delete=1`, `tokenize="trigram remove_diacritics 1"`. Each row has the rowid of
  its `pages` row (`LastInsertId()` of the `pages` insert). It holds the text unescaped, so a
  query never matches inside an HTML entity of the stored text. `searchSchemaVersion` is 3
  ([ADR-0022](./0022-versioned-derived-search-index.md)).
- Every write path changes both tables in one transaction: `IndexPages` (and `IndexPage`)
  deletes the old rows and inserts both, `RemovePages`, `RemovePage` and `RemovePageByFilePath`
  go through one `pageRowDeleter` that reads the rowids and deletes them from both tables.
  `Clear` recreates both through `searchSchema`.
- A literal-only query (no column filter, `AND`/`OR`/`NOT`, `NEAR`, quoted phrase, `term*` or
  parentheses) gets a substring part (`substringOnlyWhere`): every word of at least three runes
  must occur in the page's text (`pages_trigram MATCH`), every shorter word must still match as
  a token prefix, and pages the token query already matches are left out.
- Substring-only matches always rank after every token match, in one more tier after the
  whole-word and prefix tiers, by `bm25` over `pages_trigram` with the same column weights. The
  count covers both. `Search` ranks both parts in one `UNION ALL` of rowids, tiers and scores
  and reads the page text only for the rows of the returned page. `SearchPageIDs` returns the
  substring-only IDs after the token IDs, which are in bm25 order without tiers; its caller (the
  tag facets) uses them as a set.
- The token path is unchanged: same SQL, same highlight and snippet. For substring-only
  matches, which have no `highlight()`/`snippet()` (contentless table), `substring_highlight.go`
  marks the hit in the title and in a window of about 16 words of the content, escaped once
  like the token path.
- Rowid conditions on `pages_trigram` are written `+rowid` so that SQLite drives the query
  from the trigram MATCH: as a rowid lookup, FTS5 evaluates the MATCH once per row (measured:
  seconds instead of milliseconds on 3000 pages).

## Alternatives considered

- **A `LIKE '%word%'` fallback when the token query finds nothing:** in a real wiki a word like
  `syntax` almost always has some token hit, so the fallback would almost never run and the
  page from #1826 would stay missing. A full scan also grows with the wiki.
- **More or fewer token characters:** splitting on `-` and `_` would fix `Markdown-Syntax`, but
  not compounds or camelCase, and it changes the matching of every existing query
  (`e-mail`, `v0.14.2`, `C++`).
- **Mixing substring hits into the bm25 order:** a page that merely contains `set` inside
  `asset` would outrank a page about "set". Strict tiers keep the old results on top and only
  add pages below them.
- **A substring part for operator queries too:** there the user chose the matching (`title:`,
  `"phrase"`, `term*`); rewriting it would make those queries unpredictable.
- **An external-content trigram table (`content='pages'`):** it would need the same columns as
  `pages` and its stored text is escaped for `snippet()`; contentless is smaller and needs no
  text back.

## Consequences

- `search.db` grows by the trigram index, about 2.5 times in total on a synthetic 3000-page
  wiki (14.6 MB → 37 MB). Indexing writes a second table per page; on a synthetic 1500-page wiki
  the boot reindex took as long as before (about 29 s on a loaded 8-core machine, dominated by
  the per-page `pageID` lookup that both versions do). A substring query on a wiki where every
  page is a substring hit took about 150 ms there, a token-only query a few ms more than before
  (the substring count runs for every literal word of three or more runes).
- Short substring words (three letters) can match many pages; they only ever add results below
  the token matches, so the first results are what they were.
- Upgrading rebuilds search.db once (schema version 3). A downgrade finds an unknown schema and
  rebuilds as well.
- Terms of one or two runes, column filters and other operator queries get no substring
  matching. Revisit if users ask for `title:` substring matching, or if index size on small
  devices becomes a problem (an opt-out flag would then be added through the composition root).
