const TAGS_KEY_PATTERN = /^tags\s*:\s*(.*)$/
const FRONTMATTER_KEY_PATTERN = /^([^:\n][^:\n]*?)\s*:\s*(.*)$/
const INTERNAL_FIELD_PREFIX = 'leafwiki_'

export type EditorFrontmatterFieldType = 'text' | 'number' | 'boolean' | 'list'

export type EditorFrontmatterField = {
  key: string
  value: string
  type: EditorFrontmatterFieldType
  internal?: boolean
}

export type ParsedEditorFrontmatter = {
  tags: string[]
  fields: EditorFrontmatterField[]
  unsupportedRaw: string
}

function normalizeTag(tag: string) {
  return tag.trim()
}

function normalizeFieldKey(key: string) {
  const trimmed = key.trim()
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1).trim()
  }
  return trimmed
}

function normalizeListValue(value: string) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
    .join('\n')
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

export function normalizeEditorFrontmatterFields(
  fields: EditorFrontmatterField[],
) {
  const seen = new Set<string>()
  const result: EditorFrontmatterField[] = []

  for (const field of fields) {
    const key = normalizeFieldKey(field.key)
    if (!key) continue

    const dedupeKey = key.toLocaleLowerCase()
    if (seen.has(dedupeKey)) continue
    seen.add(dedupeKey)

    const normalizedValue =
      field.type === 'list'
        ? normalizeListValue(field.value)
        : field.value.trim()

    result.push({
      key,
      type: field.type,
      internal: field.internal,
      value:
        field.type === 'boolean'
          ? normalizedValue === 'false'
            ? 'false'
            : 'true'
          : normalizedValue,
    })
  }

  return result
}

function parseInlineList(value: string) {
  const trimmed = value.trim()
  if (!trimmed.startsWith('[') || !trimmed.endsWith(']')) {
    return null
  }

  return trimmed
    .slice(1, -1)
    .split(',')
    .map((part) => part.trim().replace(/^['"]|['"]$/g, ''))
    .filter(Boolean)
}

function detectFieldType(value: string): EditorFrontmatterFieldType {
  const trimmed = value.trim()
  if (trimmed === 'true' || trimmed === 'false') {
    return 'boolean'
  }

  if (trimmed !== '' && !Number.isNaN(Number(trimmed))) {
    return 'number'
  }

  return 'text'
}

function isInternalFieldKey(key: string) {
  return key.toLocaleLowerCase().startsWith(INTERNAL_FIELD_PREFIX)
}

function formatFieldKey(key: string) {
  const trimmed = normalizeFieldKey(key)
  if (/^[A-Za-z0-9_.-]+$/.test(trimmed)) {
    return trimmed
  }

  return JSON.stringify(trimmed)
}

function appendBlock(target: string[], header: string, bodyLines: string[]) {
  target.push(header)
  target.push(...bodyLines)
}

export function parseEditorFrontmatter(
  frontmatter?: string | null,
): ParsedEditorFrontmatter {
  const source = frontmatter?.trim() ?? ''
  if (!source) {
    return { tags: [], fields: [], unsupportedRaw: '' }
  }

  const lines = source.split('\n')
  const unsupportedLines: string[] = []
  const fields: EditorFrontmatterField[] = []
  let parsedTags: string[] | null = null

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]

    if (line.trim() === '') continue

    if (/^\s/.test(line)) {
      unsupportedLines.push(line)
      continue
    }

    const tagMatch = line.match(TAGS_KEY_PATTERN)
    if (tagMatch && parsedTags === null) {
      const inlineTags = parseInlineList(tagMatch[1] ?? '')
      if (inlineTags !== null) {
        parsedTags = normalizeTags(inlineTags)
        continue
      }

      if ((tagMatch[1] ?? '').trim() === '') {
        const collected: string[] = []
        const listItems: string[] = []
        let nextIndex = index + 1
        let supported = true

        while (nextIndex < lines.length && /^\s/.test(lines[nextIndex])) {
          collected.push(lines[nextIndex])
          const listItem = lines[nextIndex].match(/^\s*-\s*(.+?)\s*$/)
          if (!listItem) {
            supported = false
          } else {
            listItems.push(listItem[1])
          }
          nextIndex += 1
        }

        if (supported) {
          parsedTags = normalizeTags(listItems)
        } else {
          appendBlock(unsupportedLines, line, collected)
        }

        index = nextIndex - 1
        continue
      }
    }

    const keyMatch = line.match(FRONTMATTER_KEY_PATTERN)
    if (!keyMatch) {
      unsupportedLines.push(line)
      continue
    }

    const [, rawKey, rawValue] = keyMatch
    const key = normalizeFieldKey(rawKey)
    const trimmedValue = rawValue.trim()

    const inlineList = parseInlineList(trimmedValue)
    if (inlineList !== null) {
      fields.push({
        key,
        type: 'list',
        value: inlineList.join('\n'),
        internal: isInternalFieldKey(key),
      })
      continue
    }

    if (trimmedValue !== '') {
      fields.push({
        key,
        type: detectFieldType(trimmedValue),
        value: trimmedValue,
        internal: isInternalFieldKey(key),
      })
      continue
    }

    const collected: string[] = []
    const listItems: string[] = []
    let nextIndex = index + 1
    let supported = true

    while (nextIndex < lines.length && /^\s/.test(lines[nextIndex])) {
      collected.push(lines[nextIndex])
      const listItem = lines[nextIndex].match(/^\s*-\s*(.+?)\s*$/)
      if (!listItem) {
        supported = false
      } else {
        listItems.push(listItem[1])
      }
      nextIndex += 1
    }

    if (supported) {
      fields.push({
        key,
        type: 'list',
        value: listItems.join('\n'),
        internal: isInternalFieldKey(key),
      })
    } else {
      appendBlock(unsupportedLines, line, collected)
    }

    index = nextIndex - 1
  }

  return {
    tags: parsedTags ?? [],
    fields: normalizeEditorFrontmatterFields(fields),
    unsupportedRaw: unsupportedLines.join('\n').trim(),
  }
}

function buildFieldBlock(field: EditorFrontmatterField) {
  const key = normalizeFieldKey(field.key)
  if (!key) return ''
  const formattedKey = formatFieldKey(key)

  if (field.type === 'list') {
    const items = normalizeListValue(field.value).split('\n').filter(Boolean)

    if (items.length === 0) {
      return `${formattedKey}: []`
    }

    return [formattedKey + ':', ...items.map((item) => `  - ${item}`)].join(
      '\n',
    )
  }

  if (field.type === 'boolean') {
    return `${formattedKey}: ${field.value === 'false' ? 'false' : 'true'}`
  }

  return `${formattedKey}: ${field.value.trim()}`
}

export function buildEditorFrontmatter({
  tags,
  fields,
  unsupportedRaw,
}: ParsedEditorFrontmatter): string {
  const normalizedTags = normalizeTags(tags)
  const normalizedFields = normalizeEditorFrontmatterFields(fields)
  const trimmedUnsupportedRaw = unsupportedRaw.trim()
  const parts: string[] = []

  if (normalizedTags.length > 0) {
    parts.push(
      ['tags:', ...normalizedTags.map((tag) => `  - ${tag}`)].join('\n'),
    )
  }

  for (const field of normalizedFields) {
    const block = buildFieldBlock(field)
    if (block) {
      parts.push(block)
    }
  }

  if (trimmedUnsupportedRaw) {
    parts.push(trimmedUnsupportedRaw)
  }

  return parts.join('\n\n').trim()
}
