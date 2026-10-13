import { describe, expect, it } from 'vitest'
import { buildLineDiff } from './lineDiff'

const ctx = (value: string, oldLine: number, newLine: number) => ({
  kind: 'context',
  value,
  oldLineNumber: oldLine,
  newLineNumber: newLine,
})
const removed = (value: string, oldLine: number) => ({
  kind: 'removed',
  value,
  oldLineNumber: oldLine,
  newLineNumber: null,
})
const added = (value: string, newLine: number) => ({
  kind: 'added',
  value,
  oldLineNumber: null,
  newLineNumber: newLine,
})

describe('buildLineDiff', () => {
  it('reports identical content as context only', () => {
    expect(buildLineDiff('a\nb', 'a\nb')).toEqual({
      lines: [ctx('a', 1, 1), ctx('b', 2, 2)],
      summary: { addedLines: 0, removedLines: 0 },
    })
  })

  it('emits a replaced line as removal before addition', () => {
    expect(buildLineDiff('a\nb\nc', 'a\nx\nc')).toEqual({
      lines: [ctx('a', 1, 1), removed('b', 2), added('x', 2), ctx('c', 3, 3)],
      summary: { addedLines: 1, removedLines: 1 },
    })
  })

  it('numbers appended and trailing removed lines', () => {
    expect(buildLineDiff('a', 'a\nb\nc')).toEqual({
      lines: [ctx('a', 1, 1), added('b', 2), added('c', 3)],
      summary: { addedLines: 2, removedLines: 0 },
    })
    expect(buildLineDiff('a\nb\nc', 'a')).toEqual({
      lines: [ctx('a', 1, 1), removed('b', 2), removed('c', 3)],
      summary: { addedLines: 0, removedLines: 2 },
    })
  })

  it('treats an empty document as a single empty line', () => {
    expect(buildLineDiff('', 'a')).toEqual({
      lines: [removed('', 1), added('a', 1)],
      summary: { addedLines: 1, removedLines: 1 },
    })
  })

  it('prefers removing on equal-length LCS branches', () => {
    expect(buildLineDiff('x\ny', 'y\nx')).toEqual({
      lines: [removed('x', 1), ctx('y', 2, 1), added('x', 2)],
      summary: { addedLines: 1, removedLines: 1 },
    })
  })

  it('keeps the longest common subsequence as context', () => {
    const result = buildLineDiff('a\nb\nc\nd', 'b\nc\ne')
    expect(result.lines).toEqual([
      removed('a', 1),
      ctx('b', 2, 1),
      ctx('c', 3, 2),
      removed('d', 4),
      added('e', 3),
    ])
    expect(result.summary).toEqual({ addedLines: 1, removedLines: 2 })
  })
})
