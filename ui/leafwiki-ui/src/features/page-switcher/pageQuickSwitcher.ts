import { FlatPageSearchItem, searchFlatPageSearchItems } from '@/lib/pageSearch'

export type QuickSwitcherItem = FlatPageSearchItem

export function searchQuickSwitcherItems(
  items: QuickSwitcherItem[],
  query: string,
  limit = 20,
) {
  if (!query.trim()) {
    return items.slice(0, limit)
  }

  return searchFlatPageSearchItems(items, query, limit)
}
