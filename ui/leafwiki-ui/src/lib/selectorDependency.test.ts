import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { createRequire } from 'node:module'
const require = createRequire(import.meta.url)
describe('selector parser security dependency', () => {
  it('locks every selector parser copy to a patched version', () => {
    const lock = JSON.parse(readFileSync('package-lock.json', 'utf8')) as {
      packages: Record<string, { version?: string }>
    }
    const copies = Object.entries(lock.packages).filter(([path]) =>
      path.endsWith('/postcss-selector-parser'),
    )
    expect(copies.length).toBeGreaterThan(0)
    for (const [path, entry] of copies) {
      const [major, minor, patch] = entry.version!.split('.').map(Number)
      expect(
        major > 7 ||
          (major === 7 && (minor > 1 || (minor === 1 && patch >= 6))),
        path,
      ).toBe(true)
    }
  })
  it('parses a long flat selector without changing its nodes', () => {
    const parser = require('postcss-selector-parser') as (
      callback?: (root: { nodes: { nodes: unknown[] }[] }) => void,
    ) => { processSync: (input: string) => string }
    const input = '.a'.repeat(5000)
    let count = 0
    expect(
      parser((root) => {
        count = root.nodes[0].nodes.length
      }).processSync(input),
    ).toBe(input)
    expect(count).toBe(5000)
  })
})
