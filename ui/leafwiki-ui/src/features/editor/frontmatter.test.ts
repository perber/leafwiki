import { describe, expect, it } from 'vitest'
import { buildEditorFrontmatter, parseEditorFrontmatter } from './frontmatter'

// ─── parseEditorFrontmatter: nested YAML → dot-notation keys ─────────────────

describe('parseEditorFrontmatter – nested maps', () => {
  it('parses one level of nesting as a dot-notation key', () => {
    const result = parseEditorFrontmatter('a:\n  b: value')
    expect(result.fields).toHaveLength(1)
    expect(result.fields[0]).toMatchObject({
      key: 'a.b',
      value: 'value',
      type: 'text',
    })
    expect(result.unsupportedRaw).toBe('')
  })

  it('parses two levels of nesting', () => {
    const result = parseEditorFrontmatter('a:\n  b:\n    c: deep')
    expect(result.fields).toHaveLength(1)
    expect(result.fields[0]).toMatchObject({
      key: 'a.b.c',
      value: 'deep',
      type: 'text',
    })
  })

  it('parses multiple children of the same parent', () => {
    const result = parseEditorFrontmatter('a:\n  b: val1\n  c: val2')
    const keys = result.fields.map((f) => f.key)
    expect(keys).toContain('a.b')
    expect(keys).toContain('a.c')
    expect(result.fields).toHaveLength(2)
  })

  it('mixes flat and nested fields', () => {
    const result = parseEditorFrontmatter('status: draft\na:\n  b: nested')
    const keys = result.fields.map((f) => f.key)
    expect(keys).toContain('status')
    expect(keys).toContain('a.b')
    expect(result.fields).toHaveLength(2)
  })

  it('parses a block list under a key as a list field (not unsupportedRaw)', () => {
    const result = parseEditorFrontmatter('a:\n  - item1\n  - item2')
    expect(result.fields[0]).toMatchObject({ key: 'a', type: 'list' })
    expect(result.unsupportedRaw).toBe('')
  })

  it('parses inline list under a nested key', () => {
    const result = parseEditorFrontmatter('a:\n  b: [x, y, z]')
    expect(result.fields).toHaveLength(1)
    expect(result.fields[0]).toMatchObject({ key: 'a.b', type: 'list' })
  })
})

// ─── buildEditorFrontmatter: dot-notation keys → nested YAML ─────────────────

describe('buildEditorFrontmatter – nested YAML output', () => {
  it('writes a single dot-notation key as nested YAML', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [{ key: 'a.b', value: 'value', type: 'text' }],
      unsupportedRaw: '',
    })
    expect(result).toBe('a:\n  b: value')
  })

  it('writes two-level deep dot-notation key', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [{ key: 'a.b.c', value: 'deep', type: 'text' }],
      unsupportedRaw: '',
    })
    expect(result).toBe('a:\n  b:\n    c: deep')
  })

  it('groups sibling dot-notation keys under one parent block', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [
        { key: 'a.b', value: 'val1', type: 'text' },
        { key: 'a.c', value: 'val2', type: 'text' },
      ],
      unsupportedRaw: '',
    })
    // Should produce a single "a:" block, not two separate ones
    const lines = result.split('\n')
    expect(lines.filter((l) => l === 'a:')).toHaveLength(1)
    expect(result).toContain('  b: val1')
    expect(result).toContain('  c: val2')
  })

  it('mixes flat and nested keys correctly', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [
        { key: 'status', value: 'draft', type: 'text' },
        { key: 'a.b', value: 'nested', type: 'text' },
      ],
      unsupportedRaw: '',
    })
    expect(result).toContain('status: draft')
    expect(result).toContain('a:')
    expect(result).toContain('  b: nested')
  })

  it('quotes values that need YAML quoting inside nested blocks', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [{ key: 'a.b', value: 'true', type: 'text' }],
      unsupportedRaw: '',
    })
    expect(result).toContain('  b: "true"')
  })
})

// ─── Conflict and edge-case tests ────────────────────────────────────────────

describe('buildEditorFrontmatter – conflict and edge cases', () => {
  it('conflict: scalar "a" + nested "a.b" — nested falls back to flat literal key', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [
        { key: 'a', value: 'scalar', type: 'text' },
        { key: 'a.b', value: 'nested', type: 'text' },
      ],
      unsupportedRaw: '',
    })
    // "a" stays as a scalar; "a.b" is serialized as a literal flat key
    expect(result).toContain('a: scalar')
    expect(result).toContain('a.b: nested')
  })

  it('trailing-dot key "a." is treated as key "a" without crashing', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [{ key: 'a.', value: 'value', type: 'text' }],
      unsupportedRaw: '',
    })
    expect(result).toBe('a: value')
  })

  it('list field in conflict fallback is serialized as block list', () => {
    const result = buildEditorFrontmatter({
      tags: [],
      fields: [
        { key: 'a.b', value: 'val', type: 'text' },
        { key: 'a', value: 'item1\nitem2', type: 'list' },
      ],
      unsupportedRaw: '',
    })
    // The list field "a" conflicts with mapping "a.b" — falls back to block list
    expect(result).toContain('a:\n  - item1\n  - item2')
  })
})

// ─── Round-trip tests ─────────────────────────────────────────────────────────

describe('round-trip: parse → build → parse', () => {
  it('preserves a single nested field', () => {
    const original = {
      tags: [],
      fields: [{ key: 'a.b', value: 'v', type: 'text' as const }],
      unsupportedRaw: '',
    }
    const built = buildEditorFrontmatter(original)
    const parsed = parseEditorFrontmatter(built)
    expect(parsed.fields).toHaveLength(1)
    expect(parsed.fields[0]).toMatchObject({ key: 'a.b', value: 'v' })
  })

  it('preserves multiple siblings under the same parent', () => {
    const original = {
      tags: [],
      fields: [
        { key: 'a.b', value: 'val1', type: 'text' as const },
        { key: 'a.c', value: 'val2', type: 'text' as const },
      ],
      unsupportedRaw: '',
    }
    const built = buildEditorFrontmatter(original)
    const parsed = parseEditorFrontmatter(built)
    const keys = parsed.fields.map((f) => f.key).sort()
    expect(keys).toEqual(['a.b', 'a.c'])
  })

  it('preserves two-level deep nesting', () => {
    const original = {
      tags: [],
      fields: [{ key: 'a.b.c', value: 'deep', type: 'text' as const }],
      unsupportedRaw: '',
    }
    const built = buildEditorFrontmatter(original)
    const parsed = parseEditorFrontmatter(built)
    expect(parsed.fields[0]).toMatchObject({ key: 'a.b.c', value: 'deep' })
  })

  it('round-trips a boolean-value nested field with YAML quoting', () => {
    const original = {
      tags: [],
      fields: [{ key: 'a.b', value: 'true', type: 'boolean' as const }],
      unsupportedRaw: '',
    }
    const built = buildEditorFrontmatter(original)
    expect(built).toContain('  b: "true"')
    const parsed = parseEditorFrontmatter(built)
    expect(parsed.fields[0]).toMatchObject({ key: 'a.b', value: 'true' })
  })
})
