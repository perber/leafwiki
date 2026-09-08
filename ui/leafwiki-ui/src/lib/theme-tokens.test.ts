/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

/**
 * `--muted` is a *surface* token (consumed via `bg-muted`) and
 * `--muted-foreground` is a *text* token. They must never resolve to the same
 * color: when they did, `bg-muted` blocks (e.g. the shadcn `TabsList` segmented
 * toggle in the New User dialog, the sonner toast cancel button) rendered as a
 * heavy fill with their inactive label sitting on near-identical color.
 *
 * `index.css` defines both tokens once per theme block (`:root`, `.dark`, and
 * the forced-light `@media print` override). Pair them in document order and
 * assert each pair differs.
 */
// Vitest runs with cwd set to the package root (ui/leafwiki-ui).
const cssText = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

function valuesOf(prop: 'muted' | 'muted-foreground'): string[] {
  const re = new RegExp(`--${prop}:\\s*([^;!]+?)\\s*(?:!important)?;`, 'g')
  const out: string[] = []
  for (const m of cssText.matchAll(re)) {
    out.push(m[1].trim().replace(/\s+/g, ' '))
  }
  return out
}

describe('theme tokens', () => {
  it('defines --muted separately from --muted-foreground in every theme block', () => {
    const muted = valuesOf('muted')
    const mutedForeground = valuesOf('muted-foreground')

    expect(muted.length).toBeGreaterThan(0)
    expect(muted.length).toBe(mutedForeground.length)

    muted.forEach((value, i) => {
      expect(
        value,
        `--muted and --muted-foreground are identical ("${value}") in theme block #${i + 1}`,
      ).not.toBe(mutedForeground[i])
    })
  })
})
