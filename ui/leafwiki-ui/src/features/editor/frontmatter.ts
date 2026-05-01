const TAGS_KEY_PATTERN = /^tags\s*:\s*(.*)$/

export type ParsedEditorFrontmatter = {
  tags: string[]
  raw: string
}

function normalizeTag(tag: string) {
  return tag.trim()
}

export function normalizeTags(tags: string[]) {
  const seen = new Set<string>()
  const result: string[] = []

  for (const tag of tags.map(normalizeTag).filter(Boolean)) {
    const key = tag.toLocaleLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    result.push(tag)
  }

  return result
}

function parseInlineTags(value: string) {
  const trimmed = value.trim()
  if (!trimmed.startsWith('[') || !trimmed.endsWith(']')) {
    return null
  }

  return normalizeTags(
    trimmed
      .slice(1, -1)
      .split(',')
      .map((part) => part.trim().replace(/^['"]|['"]$/g, ''))
      .filter(Boolean),
  )
}

export function parseEditorFrontmatter(
  frontmatter?: string | null,
): ParsedEditorFrontmatter {
  const source = frontmatter?.trim() ?? ''
  if (!source) {
    return { tags: [], raw: '' }
  }

  const lines = source.split('\n')
  const kept: string[] = []
  let parsedTags: string[] | null = null

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    const match = line.match(TAGS_KEY_PATTERN)

    if (!match || parsedTags !== null) {
      kept.push(line)
      continue
    }

    const inlineTags = parseInlineTags(match[1] ?? '')
    if (inlineTags !== null) {
      parsedTags = inlineTags
      continue
    }

    if ((match[1] ?? '').trim() !== '') {
      kept.push(line)
      continue
    }

    const collected: string[] = []
    let nextIndex = index + 1

    while (nextIndex < lines.length) {
      const nextLine = lines[nextIndex]
      const trimmed = nextLine.trim()

      if (!trimmed) {
        collected.push(nextLine)
        nextIndex += 1
        continue
      }

      const listItem = nextLine.match(/^\s*-\s*(.+?)\s*$/)
      if (!listItem) {
        break
      }

      collected.push(listItem[1])
      nextIndex += 1
    }

    const normalized = normalizeTags(collected)
    if (
      normalized.length === 0 &&
      collected.some((line) => line.trim() === '')
    ) {
      kept.push(line)
      continue
    }

    parsedTags = normalized
    index = nextIndex - 1
  }

  return {
    tags: parsedTags ?? [],
    raw: kept.join('\n').trim(),
  }
}

export function buildEditorFrontmatter({
  tags,
  raw,
}: ParsedEditorFrontmatter): string {
  const normalizedTags = normalizeTags(tags)
  const trimmedRaw = raw.trim()
  const parts: string[] = []

  if (normalizedTags.length > 0) {
    parts.push(
      ['tags:', ...normalizedTags.map((tag) => `  - ${tag}`)].join('\n'),
    )
  }

  if (trimmedRaw) {
    parts.push(trimmedRaw)
  }

  return parts.join('\n\n').trim()
}
