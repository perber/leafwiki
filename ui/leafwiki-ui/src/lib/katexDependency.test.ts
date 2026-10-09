import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { describe, expect, it } from 'vitest'
const require = createRequire(import.meta.url)
type Renderer = {
  renderToString: (input: string, options?: Record<string, unknown>) => string
}
describe('KaTeX dependency security', () => {
  it('locks all direct and transitive renderers to patched versions', () => {
    const lock = JSON.parse(readFileSync('package-lock.json', 'utf8')) as {
      packages: Record<string, { version?: string }>
    }
    const copies = Object.entries(lock.packages).filter(
      ([path]) => path.endsWith('/katex') && !path.endsWith('/@types/katex'),
    )
    expect(copies.length).toBeGreaterThan(0)
    for (const [path, entry] of copies) {
      const [major, minor, patch] = entry.version!.split('.').map(Number)
      expect(
        major > 0 || minor > 18 || (minor === 18 && patch >= 2),
        path,
      ).toBe(true)
    }
  })
  for (const consumer of [
    'rehype-katex',
    'micromark-extension-math',
    'mermaid',
  ]) {
    it(`${consumer} ignores inherited trust and still renders ordinary math`, () => {
      const renderer = createRequire(require.resolve(consumer))(
        'katex',
      ) as Renderer
      const options = Object.assign(Object.create({ trust: true }), {
        throwOnError: false,
        strict: 'ignore',
      }) as Record<string, unknown>
      const html = renderer.renderToString(
        String.raw`\href{javascript:alert(1)}{x}`,
        options,
      )
      expect(html).not.toContain('href="javascript:')
      expect(renderer.renderToString('x^2')).toContain('class="katex"')
    })
  }
})
