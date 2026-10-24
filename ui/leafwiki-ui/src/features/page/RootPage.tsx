import PageViewer from '@/features/viewer/PageViewer'
import { useTreeStore } from '@/stores/tree'

/**
 * Renders the wiki's first page at the site root.
 *
 * This deliberately does not redirect to that page's own URL. "/" is the URL
 * search engines rank and that people link to, and a client-side redirect off
 * it tells crawlers the root is not a real page - which drops it from the
 * index in favour of the page's own path. The server renders the same page
 * into the initial HTML for "/" and points the canonical of both URLs at the
 * root, so this only has to agree with it.
 */
export default function RootPage() {
  const { tree } = useTreeStore()

  if (!tree || !tree.children || tree.children.length === 0) return null

  const first = tree.children[0]
  return <PageViewer routePath={`/${first.path}`} />
}
