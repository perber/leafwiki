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

  // Edge cases pinned so a future algorithm change is visible. Current
  // behaviour: split on '\n' only, no CRLF normalisation.
  it.each([
    {
      name: 'identical empty documents',
      base: '',
      target: '',
      lines: [ctx('', 1, 1)],
      summary: { addedLines: 0, removedLines: 0 },
    },
    {
      name: 'trailing newline only on the target side',
      base: 'a',
      target: 'a\n',
      lines: [ctx('a', 1, 1), added('', 2)],
      summary: { addedLines: 1, removedLines: 0 },
    },
    {
      name: 'trailing newline only on the base side',
      base: 'a\n',
      target: 'a',
      lines: [ctx('a', 1, 1), removed('', 2)],
      summary: { addedLines: 0, removedLines: 1 },
    },
    {
      name: 'CRLF vs LF keeps the carriage return and differs per line',
      base: 'a\r\nb',
      target: 'a\nb',
      lines: [removed('a\r', 1), added('a', 1), ctx('b', 2, 2)],
      summary: { addedLines: 1, removedLines: 1 },
    },
    {
      name: 'identical CRLF documents',
      base: 'a\r\nb\r\n',
      target: 'a\r\nb\r\n',
      lines: [ctx('a\r', 1, 1), ctx('b\r', 2, 2), ctx('', 3, 3)],
      summary: { addedLines: 0, removedLines: 0 },
    },
    {
      name: 'Unicode lines (umlauts, CJK, emoji, combining marks)',
      base: 'Grüße\n日本語\n👩‍💻 dev\ne\u0301',
      target: 'Grüße\n中文\n👩‍💻 dev\n\u00e9',
      lines: [
        ctx('Grüße', 1, 1),
        removed('日本語', 2),
        added('中文', 2),
        ctx('👩‍💻 dev', 3, 3),
        removed('e\u0301', 4),
        added('\u00e9', 4),
      ],
      summary: { addedLines: 2, removedLines: 2 },
    },
    {
      name: 'completely different documents',
      base: 'a\nb\nc',
      target: 'x\ny',
      lines: [
        removed('a', 1),
        removed('b', 2),
        removed('c', 3),
        added('x', 1),
        added('y', 2),
      ],
      summary: { addedLines: 2, removedLines: 3 },
    },
  ])('$name', ({ base, target, lines, summary }) => {
    expect(buildLineDiff(base, target)).toEqual({ lines, summary })
  })

  it('handles a large, fully different document', () => {
    const base = Array.from({ length: 1000 }, (_, i) => `old ${i}`).join('\n')
    const target = Array.from({ length: 800 }, (_, i) => `new ${i}`).join('\n')
    const { lines, summary } = buildLineDiff(base, target)
    expect(summary).toEqual({ addedLines: 800, removedLines: 1000 })
    expect(lines).toHaveLength(1800)
    expect(lines.at(-1)).toEqual(added('new 799', 800))
  })
})
