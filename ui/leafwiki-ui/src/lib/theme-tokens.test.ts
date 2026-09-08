/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

/**
 * Token roles in `index.css`:
 *   --muted           text/icon color, consumed via `text-muted` / `border-muted`
 *                     (~160 usages: tree headers, chevrons, panel labels, ...).
 *   --muted-surface   subtle fill, consumed via `bg-muted-surface`
 *                     (shadcn TabsList / avatar / separators, sonner toast,
 *                     Mermaid code blocks).
 *   --muted-foreground shadcn's inactive-text color, pairs with `bg-muted-surface`.
 *
 * `--muted-surface` must never resolve to the same color as `--muted-foreground`:
 * when it did (#1541 briefly collapsed `--muted` into the fill role), `bg-muted`
 * blocks rendered as a heavy fill with their inactive label sitting on
 * near-identical color, and every `text-muted` in the app turned near-invisible.
 *
 * Each token is defined once per theme block (`:root`, `.dark`, and the
 * forced-light `@media print` override). Pair them in document order and assert
 * each surface/foreground pair differs.
 */
// Vitest runs with cwd set to the package root (ui/leafwiki-ui).
const cssText = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

function valuesOf(prop: 'muted-surface' | 'muted-foreground'): string[] {
  const re = new RegExp(`--${prop}:\\s*([^;!]+?)\\s*(?:!important)?;`, 'g')
  const out: string[] = []
  for (const m of cssText.matchAll(re)) {
    out.push(m[1].trim().replace(/\s+/g, ' '))
  }
  return out
}

describe('theme tokens', () => {
  it('defines --muted-surface separately from --muted-foreground in every theme block', () => {
    const surface = valuesOf('muted-surface')
    const foreground = valuesOf('muted-foreground')

    expect(surface.length).toBeGreaterThan(0)
    expect(surface.length).toBe(foreground.length)

    surface.forEach((value, i) => {
      expect(
        value,
        `--muted-surface and --muted-foreground are identical ("${value}") in theme block #${i + 1}`,
      ).not.toBe(foreground[i])
    })
  })
})
