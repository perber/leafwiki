import { test } from 'vitest'
import type { PageNode } from './api/pages'
import { preprocessWikilinks, buildWikiTitleIndex } from './preprocessWikilinks'
const byId: Record<string, PageNode> = Object.fromEntries(
  Array.from({ length: 10000 }, (_, i) => [
    String(i),
    {
      id: String(i),
      title: `Page ${i}`,
      path: `page-${i}`,
      slug: `page-${i}`,
      version: 'v1',
      kind: 'page',
      children: null,
    },
  ]),
)
const content = Array.from({ length: 100 }, (_, i) => `[[Page ${i}]]`).join(' ')
const index = buildWikiTitleIndex(byId)
test('100 distinct wikilink titles among 10000 pages', async ({ bench }) => {
  await bench.compare(
    bench('memoized index', () => {
      preprocessWikilinks(
        content,
        (title) => index.get(title.toLowerCase()) ?? [],
      )
    }),
    bench('index construction plus resolution', () => {
      const rebuilt = buildWikiTitleIndex(byId)
      preprocessWikilinks(
        content,
        (title) => rebuilt.get(title.toLowerCase()) ?? [],
      )
    }),
    bench('legacy scan', () => {
      preprocessWikilinks(content, (title) => {
        const lower = title.toLowerCase()
        return Object.values(byId).filter(
          (n) => n.title.toLowerCase() === lower,
        )
      })
    }),
    { time: 1000, iterations: 10 },
  )
}, 30000)
