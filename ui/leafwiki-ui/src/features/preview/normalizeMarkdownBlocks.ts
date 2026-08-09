const blockOpenPattern =
  /^(?<indent> {0,3}):::\s*(?<type>[A-Za-z][\w-]*)(?:\s+(?<title>\S.*))?\s*$/
const blockClosePattern = /^(?<indent> {0,3}):::\s*$/
const fencedCodePattern = /^(?<indent> {0,3})(?<marker>`{3,}|~{3,})(?<rest>.*)$/

const shoutoutTypeMap: Record<string, string> = {
  caution: 'warning',
  danger: 'error',
  error: 'error',
  fail: 'error',
  failed: 'error',
  failure: 'error',
  info: 'info',
  note: 'info',
  success: 'success',
  tip: 'info',
  warn: 'warning',
  warning: 'warning',
}

function normalizeBlockType(rawType: string) {
  const normalizedType = rawType.toLowerCase().replace(/_/g, '-')
  return shoutoutTypeMap[normalizedType] ?? normalizedType
}

function prefixQuoteLine(indent: string, line: string) {
  return `${indent}>${line ? ` ${line}` : ''}`
}

type FenceState = {
  markerChar: '`' | '~'
  markerLength: number
}

function getFenceInfo(line: string): FenceState | null {
  const match = line.match(fencedCodePattern)
  if (!match?.groups) {
    return null
  }

  const marker = match.groups.marker ?? ''
  const markerChar = marker[0]
  if (markerChar !== '`' && markerChar !== '~') {
    return null
  }

  return {
    markerChar,
    markerLength: marker.length,
  }
}

function getNextFenceState(line: string, currentFence: FenceState | null) {
  const nextFence = getFenceInfo(line)

  if (!currentFence) {
    return nextFence
  }

  if (
    nextFence &&
    nextFence.markerChar === currentFence.markerChar &&
    nextFence.markerLength >= currentFence.markerLength
  ) {
    return null
  }

  return currentFence
}

export function normalizeMarkdownBlocks(content: string) {
  const normalizedContent = content.replace(/\r\n/g, '\n')
  const lines = normalizedContent.split('\n')
  const output: string[] = []
  let outerFence: FenceState | null = null

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    const openMatch = line.match(blockOpenPattern)

    if (!openMatch?.groups || outerFence) {
      output.push(line)
      outerFence = getNextFenceState(line, outerFence)
      continue
    }

    const indent = openMatch.groups.indent ?? ''
    const title = openMatch.groups.title?.trim()
    const blockType = normalizeBlockType(openMatch.groups.type ?? 'info')
    const originalBlockLines = [line]
    const blockLines: string[] = []
    let closingIndex = index + 1
    let innerFence: FenceState | null = null
    let nestedDepth = 1
    let isMalformed = false

    for (; closingIndex < lines.length; closingIndex += 1) {
      const candidateLine = lines[closingIndex]
      originalBlockLines.push(candidateLine)

      if (!innerFence) {
        if (blockOpenPattern.test(candidateLine)) {
          nestedDepth += 1
          isMalformed = true
          continue
        }

        if (blockClosePattern.test(candidateLine)) {
          nestedDepth -= 1
          if (nestedDepth === 0) {
            break
          }
          continue
        }
      }

      if (!isMalformed) {
        blockLines.push(candidateLine)
      }
      innerFence = getNextFenceState(candidateLine, innerFence)
    }

    if (closingIndex >= lines.length || isMalformed) {
      output.push(...originalBlockLines)
      index = closingIndex < lines.length ? closingIndex : lines.length - 1
      continue
    }

    if (output.length > 0 && output[output.length - 1] !== '') {
      output.push('')
    }

    if (blockType === 'collapsible' || blockType === 'collapsed') {
      appendCollapsibleBlock(output, indent, blockType, blockLines, title)
    } else {
      appendShoutoutBlock(output, indent, blockType, blockLines)
    }

    const nextLine = lines[closingIndex + 1]
    if (nextLine !== undefined && nextLine !== '') {
      output.push('')
    }

    index = closingIndex
  }

  return output.join('\n')
}

function appendCollapsibleBlock(
  output: string[],
  indent: string,
  blockType: 'collapsible' | 'collapsed',
  blockLines: string[],
  title: string | undefined,
) {
  const openAttr = blockType === 'collapsible' ? ' open' : ''

  output.push(`${indent}<details class="markdown-collapsible"${openAttr}>`)

  if (title) {
    output.push(`${indent}<summary>${title}</summary>`)
    output.push('')
  }

  output.push(...blockLines)

  output.push(`${indent}</details>`)
}

function appendShoutoutBlock(
  output: string[],
  indent: string,
  blockType: string,
  blockLines: string[],
) {
  output.push(prefixQuoteLine(indent, `[!${blockType.toUpperCase()}]`))
  output.push(prefixQuoteLine(indent, ''))

  if (blockLines.length === 0) {
    output.push(prefixQuoteLine(indent, ''))
    return
  }

  for (const blockLine of blockLines) {
    output.push(prefixQuoteLine(indent, blockLine))
  }
}
