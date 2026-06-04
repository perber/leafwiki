import { PageNode } from '@/lib/api/pages'

const WIKILINK_RE = /\[\[([^\]|#\n]+?)(?:\|([^\]\n]+?))?\]\]/g

/**
 * Replaces [[Title]] wiki-link syntax with standard Markdown links before the
 * content is passed to the Markdown renderer.
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
  return content.replace(WIKILINK_RE, (_raw, target: string, alias?: string) => {
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
  })
}
