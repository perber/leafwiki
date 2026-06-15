const TAGS_KEY_PATTERN = /^tags\s*:\s*(.*)$/
const FRONTMATTER_KEY_PATTERN = /^([^:\n][^:\n]*?)\s*:\s*(.*)$/
const INTERNAL_FIELD_PREFIX = 'leafwiki_'
const MAX_FRONTMATTER_NESTING_DEPTH = 20

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

export type EditorFrontmatterValidationErrors = Record<string, string>

function normalizeTag(tag: string) {
  return tag.trim().toLocaleLowerCase()
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

function normalizeFieldValue(value: string): string {
  const trimmed = value.trim()
  if (trimmed.length >= 2) {
    if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
      return trimmed.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, '\\')
    }
    if (trimmed.startsWith("'") && trimmed.endsWith("'")) {
      return trimmed.slice(1, -1).replace(/''/g, "'")
    }
  }
  return trimmed
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

export function validateEditorFrontmatterMetadata(
  tags: string[],
  fields: EditorFrontmatterField[],
): EditorFrontmatterValidationErrors {
  const errors: EditorFrontmatterValidationErrors = {}
  const seenTags = new Set<string>()
  const seenKeys = new Map<string, number>()

  for (const tag of tags) {
    if (tag.trim() !== tag) {
      errors.tags = 'Tags must not contain leading or trailing whitespace.'
      break
    }

    if (tag.trim() === '') {
      errors.tags = 'Tags must not be empty.'
      break
    }

    const key = tag.toLocaleLowerCase()
    if (seenTags.has(key)) {
      errors.tags = 'Tags must be unique.'
      break
    }
    seenTags.add(key)
  }

  fields.forEach((field, index) => {
    if (field.internal) return

    const keyField = `properties.${index}.key`
    const valueField = `properties.${index}.value`
    const trimmedKey = field.key.trim()

    if (trimmedKey === '') {
      errors[keyField] = 'Property key must not be empty.'
      return
    }

    if (trimmedKey !== field.key) {
      errors[keyField] =
        'Property key must not contain leading or trailing whitespace.'
      return
    }

    if (trimmedKey.toLocaleLowerCase().startsWith(INTERNAL_FIELD_PREFIX)) {
      errors[keyField] = 'Property key uses a reserved prefix.'
      return
    }

    const lowerKey = trimmedKey.toLocaleLowerCase()
    if (lowerKey === 'tags' || lowerKey === 'title') {
      errors[keyField] = 'Property key is reserved.'
      return
    }

    const dedupeKey = trimmedKey.toLocaleLowerCase()
    const existingIndex = seenKeys.get(dedupeKey)
    if (existingIndex !== undefined) {
      errors[keyField] = 'Property key must be unique.'
      if (!errors[`properties.${existingIndex}.key`]) {
        errors[`properties.${existingIndex}.key`] =
          'Property key must be unique.'
      }
      return
    }
    seenKeys.set(dedupeKey, index)

    if (field.type === 'list') {
      return
    }

    if (typeof field.value !== 'string') {
      errors[valueField] =
        'Property value must be a string, number, boolean, or flat list.'
    }
  })

  return errors
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

// needsYamlQuoting returns true for values that YAML would parse as a
// non-string type (number, boolean, null) when written bare.
function needsYamlQuoting(value: string): boolean {
  if (value === 'true' || value === 'false') return true
  if (value === 'null' || value === '~') return true
  if (value !== '' && !Number.isNaN(Number(value))) return true
  return false
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

// Tries to parse indented lines as nested key-value pairs, returning fields
// with dot-joined keys (e.g. "parent.child"). Returns null if the block
// contains anything that cannot be expressed as key-value pairs.
function tryParseNestedMap(
  lines: string[],
  baseIndent: number,
  parentKey: string,
  depth = 0,
): EditorFrontmatterField[] | null {
  if (depth >= MAX_FRONTMATTER_NESTING_DEPTH) return null
  const fields: EditorFrontmatterField[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') {
      i++
      continue
    }

    const lineIndent = line.search(/\S/)
    if (lineIndent !== baseIndent) return null

    const strippedLine = line.slice(baseIndent)
    const keyMatch = strippedLine.match(FRONTMATTER_KEY_PATTERN)
    if (!keyMatch) return null

    const [, rawKey, rawValue] = keyMatch
    const key = normalizeFieldKey(rawKey)
    const trimmedValue = rawValue.trim()
    const fullKey = parentKey ? `${parentKey}.${key}` : key

    if (trimmedValue !== '') {
      const inlineList = parseInlineList(trimmedValue)
      if (inlineList !== null) {
        fields.push({
          key: fullKey,
          type: 'list',
          value: inlineList.join('\n'),
          internal: isInternalFieldKey(fullKey),
        })
      } else {
        const normalizedValue = normalizeFieldValue(trimmedValue)
        fields.push({
          key: fullKey,
          type: detectFieldType(normalizedValue),
          value: normalizedValue,
          internal: isInternalFieldKey(fullKey),
        })
      }
      i++
      continue
    }

    // Empty value — collect all deeper-indented child lines
    const childLines: string[] = []
    let j = i + 1
    while (j < lines.length) {
      const nextLine = lines[j]
      if (nextLine.trim() === '') {
        j++
        continue
      }
      const nextIndent = nextLine.search(/\S/)
      if (nextIndent <= baseIndent) break
      childLines.push(nextLine)
      j++
    }

    if (childLines.length === 0) {
      i++
      continue
    }

    const childIndent = childLines[0].search(/\S/)

    // Try as list first, then as nested map
    const listItems: string[] = []
    let isList = true
    for (const cl of childLines) {
      if (cl.trim() === '') continue
      const listItem = cl.match(/^\s*-\s*(.+?)\s*$/)
      if (!listItem) {
        isList = false
        break
      }
      listItems.push(listItem[1])
    }

    if (isList && listItems.length > 0) {
      fields.push({
        key: fullKey,
        type: 'list',
        value: listItems.join('\n'),
        internal: isInternalFieldKey(fullKey),
      })
    } else {
      const nestedFields = tryParseNestedMap(
        childLines,
        childIndent,
        fullKey,
        depth + 1,
      )
      if (nestedFields === null) {
        // This child block cannot be expressed as key-value pairs.
        // Abort the whole parent block so the caller can fall back to
        // unsupportedRaw — otherwise partial siblings would be silently lost.
        return null
      }
      fields.push(...nestedFields)
    }

    i = j
  }

  return fields
}

// ─── Field tree for nested YAML serialization ─────────────────────────────────

type FieldTreeNode = {
  field?: EditorFrontmatterField
  children: Map<string, FieldTreeNode>
}

// addToFieldTree returns false when inserting would cause a YAML-illegal
// conflict (a key being both a scalar and a mapping at the same path).
function addToFieldTree(
  tree: Map<string, FieldTreeNode>,
  parts: string[],
  field: EditorFrontmatterField,
  depth = 0,
): boolean {
  if (depth >= MAX_FRONTMATTER_NESTING_DEPTH) return false
  const [head, ...rest] = parts
  if (!head) return false

  if (!tree.has(head)) {
    tree.set(head, { children: new Map() })
  }
  const node = tree.get(head)!

  if (rest.length === 0) {
    // Conflict: node is already a parent, or an identical-path field was already inserted.
    if (node.children.size > 0 || node.field !== undefined) return false
    node.field = field
    return true
  }

  if (node.field !== undefined) return false // would conflict with existing scalar
  return addToFieldTree(node.children, rest, field, depth + 1)
}

function serializeLeafField(
  formattedKey: string,
  field: EditorFrontmatterField,
  indent: number,
): string {
  const prefix = '  '.repeat(indent)
  if (field.type === 'list') {
    const items = normalizeListValue(field.value).split('\n').filter(Boolean)
    if (items.length === 0) return `${prefix}${formattedKey}: []`
    return [
      `${prefix}${formattedKey}:`,
      ...items.map((item) => `${prefix}  - ${item}`),
    ].join('\n')
  }
  const trimmedValue = field.value.trim()
  if (needsYamlQuoting(trimmedValue)) {
    return `${prefix}${formattedKey}: "${trimmedValue}"`
  }
  return `${prefix}${formattedKey}: ${trimmedValue}`
}

function serializeFieldNode(
  key: string,
  node: FieldTreeNode,
  indent: number,
): string {
  const prefix = '  '.repeat(indent)
  const formattedKey = formatFieldKey(key)

  if (node.children.size > 0) {
    const childLines: string[] = []
    for (const [childKey, childNode] of node.children) {
      const serialized = serializeFieldNode(childKey, childNode, indent + 1)
      if (serialized) childLines.push(serialized)
    }
    if (childLines.length === 0) return ''
    return `${prefix}${formattedKey}:\n${childLines.join('\n')}`
  }

  if (!node.field) return ''
  return serializeLeafField(formattedKey, node.field, indent)
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
      const normalizedValue = normalizeFieldValue(trimmedValue)
      fields.push({
        key,
        type: detectFieldType(normalizedValue),
        value: normalizedValue,
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
      const firstChild = collected.find((l) => l.trim() !== '')
      const nestedIndent = firstChild ? firstChild.search(/\S/) : 2
      const nestedFields = tryParseNestedMap(collected, nestedIndent, key)
      if (nestedFields !== null) {
        fields.push(...nestedFields)
      } else {
        appendBlock(unsupportedLines, line, collected)
      }
    }

    index = nextIndex - 1
  }

  return {
    tags: parsedTags ?? [],
    fields: normalizeEditorFrontmatterFields(fields),
    unsupportedRaw: unsupportedLines.join('\n').trim(),
  }
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

  // Build a tree so that dot-notation keys (e.g. "a.b" and "a.c") are grouped
  // under a single parent block instead of emitting duplicate YAML keys.
  // Fields that cannot be nested without conflict (e.g. "a" alongside "a.b")
  // are serialized as flat literal YAML keys instead.
  const fieldTree = new Map<string, FieldTreeNode>()
  const flatFallbacks: EditorFrontmatterField[] = []

  for (const field of normalizedFields) {
    const key = normalizeFieldKey(field.key)
    if (!key) continue
    // Filter empty segments — handles trailing/leading/consecutive dots.
    const parts = key.split('.').filter(Boolean)
    if (parts.length === 0) continue
    if (!addToFieldTree(fieldTree, parts, field)) {
      flatFallbacks.push({ ...field, key })
    }
  }

  for (const [key, node] of fieldTree) {
    const serialized = serializeFieldNode(key, node, 0)
    if (serialized) parts.push(serialized)
  }

  for (const field of flatFallbacks) {
    parts.push(serializeLeafField(formatFieldKey(field.key), field, 0))
  }

  if (trimmedUnsupportedRaw) {
    parts.push(trimmedUnsupportedRaw)
  }

  return parts.join('\n\n').trim()
}
