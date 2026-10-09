import { test } from 'vitest'
import {
  searchFlatPageSearchItems,
  type FlatPageSearchItem,
} from './pageSearch'
const items: FlatPageSearchItem[] = Array.from({ length: 10000 }, (_, i) => ({
  id: String(i),
  title: `Page ${i}`,
  path: `docs/${i}`,
  kind: 'page',
  breadcrumb: `Docs / Page ${i}`,
  searchText: `page ${i} docs alpha beta`,
  normalizedTitle: `page ${i}`,
  normalizedPath: `docs/${i}`,
  normalizedBreadcrumb: `docs / page ${i}`,
}))
test('flat page search: 10000 candidates', async ({ bench }) => {
  await bench.compare(
    bench('multiword fallback', () => {
      searchFlatPageSearchItems(items, 'alpha beta', 20)
    }),
    bench('single-word miss', () => {
      searchFlatPageSearchItems(items, 'missing', 20)
    }),
    { time: 1000, iterations: 10 },
  )
}, 30000)
