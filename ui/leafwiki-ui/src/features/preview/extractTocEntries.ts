import { slugifyHeadline } from './rehypeLineNumber'

export type TocEntry = {
  level: 1 | 2 | 3
  text: string
  id: string
}

function stripInlineMarkdown(text: string): string {
  return text
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/<[^>]*>/g, '')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\[[^\]]*\]/g, '$1')
    .replace(/\[([^\]]*)\](?!\(|\[)/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/[*_]{1,3}([^*_]+)[*_]{1,3}/g, '$1')
    .trim()
}

export function extractTocEntries(markdown: string): TocEntry[] {
  const lines = markdown.split('\n')
  const entries: TocEntry[] = []
  const slugCounts: Record<string, number> = {}
  let inCodeBlock = false
  let fenceChar: string | null = null
  let fenceLength = 0

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    if (inCodeBlock) {
      const closeMatch = line.match(/^ {0,3}(`{3,}|~{3,})[ \t]*$/)
      if (
        closeMatch &&
        closeMatch[1][0] === fenceChar &&
        closeMatch[1].length >= fenceLength
      ) {
        inCodeBlock = false
        fenceChar = null
        fenceLength = 0
      }
      continue
    }

    const openMatch = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/)
    if (openMatch) {
      inCodeBlock = true
      fenceChar = openMatch[1][0]
      fenceLength = openMatch[1].length
      continue
    }

    // ATX headings: match H1-H6 for correct slug duplicate counting, only add H1-H3 to entries
    const atxMatch = line.match(/^(#{1,6})\s+(.+?)(?:\s+#+\s*)?$/)
    if (atxMatch) {
      const level = atxMatch[1].length
      const text = stripInlineMarkdown(atxMatch[2])
      if (!text) continue
      const baseSlug = slugifyHeadline(text)
      if (!baseSlug) continue
      const count = slugCounts[baseSlug] ?? 0
      slugCounts[baseSlug] = count + 1
      const id = count === 0 ? baseSlug : `${baseSlug}-${count}`
      if (level <= 3) {
        entries.push({ level: level as 1 | 2 | 3, text, id })
      }
      continue
    }

    // Setext H1: line followed by ===
    const nextLine = lines[i + 1]
    if (nextLine !== undefined) {
      const rawText = line.trim()
      if (rawText && /^=+\s*$/.test(nextLine)) {
        const text = stripInlineMarkdown(rawText)
        if (text) {
          const baseSlug = slugifyHeadline(text)
          if (baseSlug) {
            const count = slugCounts[baseSlug] ?? 0
            slugCounts[baseSlug] = count + 1
            entries.push({
              level: 1,
              text,
              id: count === 0 ? baseSlug : `${baseSlug}-${count}`,
            })
          }
        }
        i++
        continue
      }
      // Setext H2: line followed by ---
      if (rawText && /^-+\s*$/.test(nextLine)) {
        const text = stripInlineMarkdown(rawText)
        if (text) {
          const baseSlug = slugifyHeadline(text)
          if (baseSlug) {
            const count = slugCounts[baseSlug] ?? 0
            slugCounts[baseSlug] = count + 1
            entries.push({
              level: 2,
              text,
              id: count === 0 ? baseSlug : `${baseSlug}-${count}`,
            })
          }
        }
        i++
      }
    }
  }

  return entries
}
