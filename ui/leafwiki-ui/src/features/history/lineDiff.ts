export type DiffLine = {
  kind: 'context' | 'added' | 'removed'
  value: string
  oldLineNumber: number | null
  newLineNumber: number | null
}

export type DiffSummary = {
  addedLines: number
  removedLines: number
}

// buildLineDiff computes a line-based diff via a full LCS table: O(n*m) time
// and memory. On equal-length LCS branches a removal is emitted before an
// addition.
export function buildLineDiff(
  baseContent: string,
  targetContent: string,
): {
  lines: DiffLine[]
  summary: DiffSummary
} {
  const baseLines = baseContent.split('\n')
  const targetLines = targetContent.split('\n')
  const rows = baseLines.length
  const cols = targetLines.length
  const matrix = new Uint32Array((rows + 1) * (cols + 1))

  const index = (row: number, col: number) => row * (cols + 1) + col

  for (let row = rows - 1; row >= 0; row -= 1) {
    for (let col = cols - 1; col >= 0; col -= 1) {
      if (baseLines[row] === targetLines[col]) {
        matrix[index(row, col)] = matrix[index(row + 1, col + 1)] + 1
      } else {
        matrix[index(row, col)] = Math.max(
          matrix[index(row + 1, col)],
          matrix[index(row, col + 1)],
        )
      }
    }
  }

  const lines: DiffLine[] = []
  let row = 0
  let col = 0
  let oldLineNumber = 1
  let newLineNumber = 1
  let addedLines = 0
  let removedLines = 0

  while (row < rows && col < cols) {
    if (baseLines[row] === targetLines[col]) {
      lines.push({
        kind: 'context',
        value: baseLines[row],
        oldLineNumber,
        newLineNumber,
      })
      row += 1
      col += 1
      oldLineNumber += 1
      newLineNumber += 1
      continue
    }

    if (matrix[index(row + 1, col)] >= matrix[index(row, col + 1)]) {
      lines.push({
        kind: 'removed',
        value: baseLines[row],
        oldLineNumber,
        newLineNumber: null,
      })
      row += 1
      oldLineNumber += 1
      removedLines += 1
      continue
    }

    lines.push({
      kind: 'added',
      value: targetLines[col],
      oldLineNumber: null,
      newLineNumber,
    })
    col += 1
    newLineNumber += 1
    addedLines += 1
  }

  while (row < rows) {
    lines.push({
      kind: 'removed',
      value: baseLines[row],
      oldLineNumber,
      newLineNumber: null,
    })
    row += 1
    oldLineNumber += 1
    removedLines += 1
  }

  while (col < cols) {
    lines.push({
      kind: 'added',
      value: targetLines[col],
      oldLineNumber: null,
      newLineNumber,
    })
    col += 1
    newLineNumber += 1
    addedLines += 1
  }

  return {
    lines,
    summary: { addedLines, removedLines },
  }
}
