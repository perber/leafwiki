import { PageNode } from '@/lib/api/pages'

const WIKILINK_RE = /\[\[([^\]|#\n]+?)(?:\|([^\]\n]+?))?\]\]/g

// Matches fenced code blocks (``` ... ```) and inline code spans (`...`).
// These are extracted before wiki-link processing so [[...]] inside code is
// never converted to a link.
const FENCED_CODE_RE = /```[\s\S]*?```/g
const INLINE_CODE_RE = /`[^`\n]+`/g
const PLACEHOLDER_RE = /\x00WLPH(\d+)\x00/g

/**
 * Replaces [[Title]] wiki-link syntax with standard Markdown links before the
 * content is passed to the Markdown renderer.
 *
 * Code blocks and inline code spans are preserved unchanged.
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
  const placeholders: string[] = []

  const makePlaceholder = (m: string) => {
    placeholders.push(m)
    return `\x00WLPH${placeholders.length - 1}\x00`
  }

  // Protect code blocks and inline code from wiki-link substitution.
  const stripped = content
    .replace(FENCED_CODE_RE, makePlaceholder)
    .replace(INLINE_CODE_RE, makePlaceholder)

  const processed = stripped.replace(
    WIKILINK_RE,
    (_raw, target: string, alias?: string) => {
      const trimmedTarget = target.trim()
      const displayText = alias ? alias.trim() : trimmedTarget

      if (trimmedTarget.includes('/')) {
        return `[${displayText}](/${trimmedTarget})`
      }

      const matches = getPagesByTitle(trimmedTarget)

      if (matches.length === 1) {
        return `[${displayText}](/${matches[0].path})`
      }

      if (matches.length === 0) {
        return `[${displayText}](wikilink-notfound:${encodeURIComponent(trimmedTarget)})`
      }

      return `[${displayText}](wikilink-ambiguous:${encodeURIComponent(trimmedTarget)})`
    },
  )

  // Restore the original code spans.
  return processed.replace(PLACEHOLDER_RE, (_, i) => placeholders[+i])
}
