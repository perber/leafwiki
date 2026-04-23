const shoutoutOpenPattern =
  /^(?<indent> {0,3}):::\s*(?<type>[A-Za-z][\w-]*)\s*$/
const shoutoutClosePattern = /^(?<indent> {0,3}):::\s*$/
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

function normalizeShoutoutType(rawType: string) {
  const normalizedType = rawType.toLowerCase()
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

export function normalizeMarkdownShoutouts(content: string) {
  const normalizedContent = content.replace(/\r\n/g, '\n')
  const lines = normalizedContent.split('\n')
  const output: string[] = []
  let outerFence: FenceState | null = null

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    const openMatch = line.match(shoutoutOpenPattern)

    if (!openMatch?.groups || outerFence) {
      output.push(line)
      outerFence = getNextFenceState(line, outerFence)
      continue
    }

    const indent = openMatch.groups.indent ?? ''
    const variant = normalizeShoutoutType(openMatch.groups.type ?? 'info')
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
        if (shoutoutOpenPattern.test(candidateLine)) {
          nestedDepth += 1
          isMalformed = true
          continue
        }

        if (shoutoutClosePattern.test(candidateLine)) {
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
      if (closingIndex < lines.length) {
        index = closingIndex
      }
      continue
    }

    if (output.length > 0 && output[output.length - 1] !== '') {
      output.push('')
    }

    output.push(prefixQuoteLine(indent, `[!${variant.toUpperCase()}]`))
    output.push(prefixQuoteLine(indent, ''))

    if (blockLines.length === 0) {
      output.push(prefixQuoteLine(indent, ''))
    } else {
      for (const blockLine of blockLines) {
        output.push(prefixQuoteLine(indent, blockLine))
      }
    }

    const nextLine = lines[closingIndex + 1]
    if (nextLine !== undefined && nextLine !== '') {
      output.push('')
    }

    index = closingIndex
  }

  return output.join('\n')
}
