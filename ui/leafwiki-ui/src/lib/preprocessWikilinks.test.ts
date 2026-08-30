import { describe, it, expect, vi } from 'vitest'
import { preprocessWikilinks } from './preprocessWikilinks'
import type { PageNode } from '@/lib/api/pages'

const page = (id: string, path: string, title: string): PageNode => ({
  id,
  title,
  slug: path.split('/').pop() ?? path,
  path,
  version: '1',
  kind: 'page',
  children: null,
})

const noMatch = () => []
const singleMatch = (p: PageNode) => () => [p]
const multiMatch = (pages: PageNode[]) => () => pages

describe('preprocessWikilinks', () => {
  describe('single match', () => {
    it('replaces [[Title]] with a resolved markdown link', () => {
      const docs = page('1', 'docs/intro', 'Intro')
      const result = preprocessWikilinks(
        'See [[Intro]] here.',
        singleMatch(docs),
      )
      expect(result).toBe('See [Intro](/docs/intro) here.')
    })

    it('uses the alias as display text for [[Title|Alias]]', () => {
      const docs = page('1', 'docs/intro', 'Intro')
      const result = preprocessWikilinks(
        '[[Intro|our intro]]',
        singleMatch(docs),
      )
      expect(result).toBe('[our intro](/docs/intro)')
    })

    it('trims trailing whitespace from target and alias', () => {
      const docs = page('1', 'docs/intro', 'Intro')
      const result = preprocessWikilinks(
        '[[Intro |our intro ]]',
        singleMatch(docs),
      )
      expect(result).toBe('[our intro](/docs/intro)')
    })
  })

  describe('no match', () => {
    it('emits wikilink-notfound: scheme when no page has that title', () => {
      const result = preprocessWikilinks('[[Missing Page]]', noMatch)
      expect(result).toBe('[Missing Page](wikilink-notfound:Missing%20Page)')
    })

    it('percent-encodes spaces and special characters in the notfound scheme', () => {
      const result = preprocessWikilinks('[[Project Plan]]', noMatch)
      expect(result).toBe('[Project Plan](wikilink-notfound:Project%20Plan)')
    })
  })

  describe('multiple matches', () => {
    it('emits wikilink-ambiguous: scheme when multiple pages match', () => {
      const matches = multiMatch([
        page('1', 'a/notes', 'Notes'),
        page('2', 'b/notes', 'Notes'),
      ])
      const result = preprocessWikilinks('[[Notes]]', matches)
      expect(result).toBe('[Notes](wikilink-ambiguous:Notes)')
    })

    it('encodes the title in the ambiguous scheme', () => {
      const matches = multiMatch([
        page('1', 'a/my-notes', 'My Notes'),
        page('2', 'b/my-notes', 'My Notes'),
      ])
      const result = preprocessWikilinks('[[My Notes]]', matches)
      expect(result).toBe('[My Notes](wikilink-ambiguous:My%20Notes)')
    })
  })

  describe('path hints ([[Folder/Title]])', () => {
    it('converts a slash-containing target with no title match to a direct path link', () => {
      const result = preprocessWikilinks('[[docs/intro]]', noMatch)
      expect(result).toBe('[docs/intro](</docs/intro>)')
    })

    it('uses the alias as display text for path hints', () => {
      const result = preprocessWikilinks('[[docs/intro|Introduction]]', noMatch)
      expect(result).toBe('[Introduction](</docs/intro>)')
    })

    // Regression: a slash in the *title* must not short-circuit the title
    // lookup. Mirrors the backend's resolveWikiLinkTargets, which always tries
    // a title match first (its motivating examples: "TCP/IP", "C/C++ Guide").
    it('resolves a slash-containing target by title when a single page matches', () => {
      const guide = page('1', 'dev/c-cpp-guide', 'C/C++ Guide')
      const result = preprocessWikilinks('[[C/C++ Guide]]', singleMatch(guide))
      expect(result).toBe('[C/C++ Guide](/dev/c-cpp-guide)')
    })

    // Regression for the ADR-0011 index bug: title contains "/" ("CLI/ENV") and
    // also spaces, so the old path-hint branch emitted a bare markdown link
    // whose destination had spaces — invalid CommonMark, rendered as raw text.
    it('resolves a slash-and-space title against the tree instead of emitting raw text', () => {
      const adr = page(
        '1',
        'ai-gen-infos/adr/adr-0011-smtp',
        'ADR-0011: SMTP as a CLI/ENV-Only Optional Feature',
      )
      const result = preprocessWikilinks(
        '[[ADR-0011: SMTP as a CLI/ENV-Only Optional Feature]]',
        singleMatch(adr),
      )
      expect(result).toBe(
        '[ADR-0011: SMTP as a CLI/ENV-Only Optional Feature](/ai-gen-infos/adr/adr-0011-smtp)',
      )
    })

    // A slash-containing target with no title match AND spaces still has to
    // produce a parseable link — angle-bracket the destination.
    it('angle-brackets an unresolved slash target so spaces stay parseable', () => {
      const result = preprocessWikilinks('[[some/nested page]]', noMatch)
      expect(result).toBe('[some/nested page](</some/nested page>)')
    })

    // The path-hint destination must not become a protocol-relative "//host"
    // URL just because the target had a leading slash.
    it('collapses leading slashes in the path-hint destination to exactly one', () => {
      const result = preprocessWikilinks('[[//evil.example/path]]', noMatch)
      expect(result).toBe('[//evil.example/path](</evil.example/path>)')
    })

    it('strips a single leading slash from the path-hint destination', () => {
      const result = preprocessWikilinks('[[/docs/intro]]', noMatch)
      expect(result).toBe('[/docs/intro](</docs/intro>)')
    })

    // "<" / ">" in the target must not terminate the <...> destination early.
    it('percent-escapes angle brackets in the path-hint destination', () => {
      const result = preprocessWikilinks('[[docs/<script>]]', noMatch)
      expect(result).toBe('[docs/<script>](</docs/%3Cscript%3E>)')
    })

    it('treats an ambiguous slash-containing title as ambiguous, not a path hint', () => {
      const matches = multiMatch([
        page('1', 'a/tcp-ip', 'TCP/IP'),
        page('2', 'b/tcp-ip', 'TCP/IP'),
      ])
      const result = preprocessWikilinks('[[TCP/IP]]', matches)
      expect(result).toBe('[TCP/IP](wikilink-ambiguous:TCP%2FIP)')
    })
  })

  describe('code block protection', () => {
    it('does not convert [[Title]] inside fenced code blocks', () => {
      const content = '```\nSee [[Intro]] for details\n```'
      const result = preprocessWikilinks(
        content,
        singleMatch(page('1', 'intro', 'Intro')),
      )
      expect(result).toBe(content)
    })

    it('does not convert [[Title]] inside inline code', () => {
      const content = 'Use `[[Title]]` syntax.'
      const result = preprocessWikilinks(
        content,
        singleMatch(page('1', 'title', 'Title')),
      )
      expect(result).toBe(content)
    })

    it('converts [[Title]] outside code blocks but leaves code intact', () => {
      const intro = page('1', 'docs/intro', 'Intro')
      const content = 'See [[Intro]].\n\n```\n[[Intro]] example\n```'
      const result = preprocessWikilinks(content, singleMatch(intro))
      expect(result).toBe(
        'See [Intro](/docs/intro).\n\n```\n[[Intro]] example\n```',
      )
    })

    it('handles multiple code spans and wiki-links interleaved', () => {
      const intro = page('1', 'docs/intro', 'Intro')
      const content = '`code1` [[Intro]] `code2`'
      const result = preprocessWikilinks(content, singleMatch(intro))
      expect(result).toBe('`code1` [Intro](/docs/intro) `code2`')
    })
  })

  describe('caching', () => {
    it('calls getPagesByTitle only once for repeated occurrences of the same title', () => {
      const intro = page('1', 'docs/intro', 'Intro')
      const getPagesByTitle = vi.fn().mockReturnValue([intro])
      preprocessWikilinks('[[Intro]] and [[Intro]] again', getPagesByTitle)
      expect(getPagesByTitle).toHaveBeenCalledTimes(1)
    })

    it('calls getPagesByTitle separately for case-variant titles', () => {
      const getPagesByTitle = vi.fn().mockReturnValue([])
      preprocessWikilinks('[[Notes]] and [[notes]]', getPagesByTitle)
      // cache key is lowercased, so both resolve from the same cache entry
      expect(getPagesByTitle).toHaveBeenCalledTimes(1)
    })
  })

  // Regression for #1200: bash [[ ... ]] conditionals must not be treated as wikilinks
  describe('bash double-bracket conditionals', () => {
    it('does not treat [[ ]] in bash conditionals as a wikilink', () => {
      const content = 'if [[ -n "$computed_hash" && -n "$src_hash" ]]; then'
      const result = preprocessWikilinks(content, noMatch)
      expect(result).toBe(content)
    })

    it('does not treat [[ ]] with == operator as a wikilink', () => {
      const content = 'if [[ "$src_hash" == sha256-* ]]; then'
      const result = preprocessWikilinks(content, noMatch)
      expect(result).toBe(content)
    })

    // [[ <space>Title ]] is intentionally not supported — leading whitespace after [[
    // is the distinguishing marker of bash conditionals and is therefore rejected.
    it('does not treat [[ Title ]] with leading space as a wikilink', () => {
      const content = '[[ Intro ]]'
      const result = preprocessWikilinks(content, noMatch)
      expect(result).toBe(content)
    })
  })

  describe('edge cases', () => {
    it('returns empty string unchanged', () => {
      expect(preprocessWikilinks('', noMatch)).toBe('')
    })

    it('returns content without wiki-links unchanged', () => {
      const content = 'Normal [link](/page) and text.'
      expect(preprocessWikilinks(content, noMatch)).toBe(content)
    })

    it('handles multiple different wiki-links in one pass', () => {
      const pageA = page('1', 'a', 'Alpha')
      const pageB = page('2', 'b', 'Beta')
      const lookup = (title: string) => {
        if (title === 'Alpha') return [pageA]
        if (title === 'Beta') return [pageB]
        return []
      }
      const result = preprocessWikilinks('[[Alpha]] and [[Beta]]', lookup)
      expect(result).toBe('[Alpha](/a) and [Beta](/b)')
    })
  })
})
