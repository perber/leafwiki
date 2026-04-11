type FenceState = {
  marker: '`' | '~'
  length: number
}

type ListContext = {
  effectiveIndent: number
  rawIndent: number
}

type ParsedListLine = {
  containerPrefix: string
  indent: string
  listMarker: string
  rest: string
  spacing: string
}

type ParsedLinePrefix = {
  containerPrefix: string
  indent: number
}

const fencePattern =
  /^(?<containerPrefix>(?: {0,3}>\s*)*)(?<indent> *)(?<marker>`{3,}|~{3,})(?<rest>.*)$/
const listPattern =
  /^(?<containerPrefix>(?: {0,3}>\s*)*)(?<indent> *)(?<listMarker>(?:[-+*])|(?:\d+[.)]))(?<spacing>\s+)(?<rest>.*)$/
const linePrefixPattern = /^(?<containerPrefix>(?: {0,3}>\s*)*)(?<indent> *)/

function parseListLine(line: string): ParsedListLine | null {
  const match = listPattern.exec(line)
  if (!match?.groups) {
    return null
  }

  return {
    containerPrefix: match.groups.containerPrefix ?? '',
    indent: match.groups.indent ?? '',
    listMarker: match.groups.listMarker ?? '',
    spacing: match.groups.spacing ?? ' ',
    rest: match.groups.rest ?? '',
  }
}

function parseLinePrefix(line: string): ParsedLinePrefix | null {
  const match = linePrefixPattern.exec(line)
  if (!match?.groups) {
    return null
  }

  return {
    containerPrefix: match.groups.containerPrefix ?? '',
    indent: (match.groups.indent ?? '').length,
  }
}

function getNextFenceState(
  line: string,
  currentFence: FenceState | null,
): FenceState | null {
  const match = fencePattern.exec(line)
  if (!match?.groups) {
    return currentFence
  }

  const markerRun = match.groups.marker
  const marker = markerRun[0] as FenceState['marker']

  if (!currentFence) {
    return {
      marker,
      length: markerRun.length,
    }
  }

  if (
    marker !== currentFence.marker ||
    markerRun.length < currentFence.length
  ) {
    return currentFence
  }

  return null
}

function clearCompletedContexts(
  line: string,
  listContexts: Map<string, ListContext[]>,
) {
  if (line.trim() === '') {
    return
  }

  const parsedPrefix = parseLinePrefix(line)
  if (!parsedPrefix) {
    listContexts.clear()
    return
  }

  for (const [containerKey, stack] of listContexts) {
    if (containerKey !== parsedPrefix.containerPrefix) {
      listContexts.delete(containerKey)
      continue
    }

    const nextStack = stack.filter(
      (context) => parsedPrefix.indent > context.rawIndent,
    )

    if (nextStack.length === 0) {
      listContexts.delete(containerKey)
      continue
    }

    listContexts.set(containerKey, nextStack)
  }
}

function normalizeListItemIndentation(
  line: string,
  listContexts: Map<string, ListContext[]>,
): string {
  const parsed = parseListLine(line)
  if (!parsed) {
    clearCompletedContexts(line, listContexts)
    return line
  }

  const rawIndent = parsed.indent.length
  const containerKey = parsed.containerPrefix
  const contextStack = [...(listContexts.get(containerKey) ?? [])]

  while (
    contextStack.length > 0 &&
    rawIndent < contextStack[contextStack.length - 1].rawIndent
  ) {
    contextStack.pop()
  }

  let effectiveIndent = rawIndent

  if (contextStack.length > 0) {
    const previousContext = contextStack[contextStack.length - 1]

    if (rawIndent === previousContext.rawIndent) {
      effectiveIndent = previousContext.effectiveIndent
    } else if (rawIndent > previousContext.rawIndent) {
      const indentStep = rawIndent - previousContext.rawIndent
      effectiveIndent =
        previousContext.effectiveIndent +
        (indentStep === 2 || indentStep === 4 ? 4 : indentStep)
    }
  }

  if (
    contextStack.length === 0 ||
    rawIndent !== contextStack[contextStack.length - 1]?.rawIndent
  ) {
    contextStack.push({
      rawIndent,
      effectiveIndent,
    })
  } else {
    contextStack[contextStack.length - 1] = {
      rawIndent,
      effectiveIndent,
    }
  }

  listContexts.set(containerKey, contextStack)

  if (effectiveIndent === rawIndent) {
    return line
  }

  return `${parsed.containerPrefix}${' '.repeat(effectiveIndent)}${parsed.listMarker}${parsed.spacing}${parsed.rest}`
}

export function normalizeMarkdownListIndentation(content: string) {
  const lines = content.split('\n')
  let fenceState: FenceState | null = null
  const listContexts = new Map<string, ListContext[]>()

  return lines
    .map((line) => {
      const activeFence = fenceState
      fenceState = getNextFenceState(line, fenceState)

      if (activeFence) {
        return line
      }

      return normalizeListItemIndentation(line, listContexts)
    })
    .join('\n')
}
