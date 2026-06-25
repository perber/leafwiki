import { PageNode } from '@/lib/api/pages'

const WIKILINK_RE = /\[\[(\S[^\]|#\n]*?)(?:\|([^\]\n]+?))?\]\]/g

// Splits content into alternating [text, code, text, code, ...] segments.
// The capture group keeps the code spans in the result array at odd indices.
const CODE_SPLIT_RE = /(```[\s\S]*?```|`[^`\n]+`)/g

/**
 * Replaces [[Title]] wiki-link syntax with standard Markdown links before the
 * content is passed to the Markdown renderer.
 *
 * Code blocks and inline code spans are preserved unchanged because only
 * the even-indexed segments (plain text) are processed by the wiki-link regex.
 * Repeated [[Title]] occurrences within a single call are resolved only once
 * (cached) to avoid O(numLinks × numPages) scans.
 *
 * Resolution rules:
 *  - [[Folder/Title]]         → [Folder/Title](/Folder/Title)  (treated as a path hint)
 *  - [[Title]]                → [Title](/path)                 (single match in tree)
 *  - [[Title|Alias]]          → [Alias](/path)                 (with custom display text)
 *  - no match                 → uses wikilink-notfound: scheme (rendered red)
 *  - multiple matches         → uses wikilink-ambiguous: scheme (opens disambiguation)
 */
export function preprocessWikilinks(
  content: string,
  getPagesByTitle: (title: string) => PageNode[],
): string {
  const cache = new Map<string, PageNode[]>()

  const getMatches = (title: string): PageNode[] => {
    const key = title.toLowerCase()
    if (!cache.has(key)) cache.set(key, getPagesByTitle(title))
    return cache.get(key)!
  }

  const replaceWikilinks = (text: string): string =>
    text.replace(WIKILINK_RE, (_raw, target: string, alias?: string) => {
      const trimmedTarget = target.trim()
      const displayText = alias ? alias.trim() : trimmedTarget

      if (trimmedTarget.includes('/')) {
        return `[${displayText}](/${trimmedTarget})`
      }

      const matches = getMatches(trimmedTarget)

      if (matches.length === 1) {
        return `[${displayText}](/${matches[0].path})`
      }

      if (matches.length === 0) {
        return `[${displayText}](wikilink-notfound:${encodeURIComponent(trimmedTarget)})`
      }

      return `[${displayText}](wikilink-ambiguous:${encodeURIComponent(trimmedTarget)})`
    })

  // Split preserves code spans at odd indices; only even indices (plain text)
  // are processed for wiki-links.
  return content
    .split(CODE_SPLIT_RE)
    .map((segment, i) => (i % 2 === 0 ? replaceWikilinks(segment) : segment))
    .join('')
}
