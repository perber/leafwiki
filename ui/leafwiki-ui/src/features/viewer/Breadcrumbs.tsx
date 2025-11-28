import { useAppMode } from '@/lib/useAppMode'
import { useTreeStore } from '@/stores/tree'
import { Link } from 'react-router-dom'
import { useViewerStore } from './viewer'

export default function Breadcrumbs() {
  const tree = useTreeStore((s) => s.tree)
  const page = useViewerStore((s) => s.page)

  const appMode = useAppMode()

  if (!page || !tree || !tree.children) return null

  if (appMode === 'edit') {
    return null
  }

  const segments = page.path.split('/').filter(Boolean)

  const buildBreadcrumbs = () => {
    const crumbs = []
    let current = tree
    let path = ''
    for (const segment of segments) {
      if (!current.children) break
      const match = current.children.find((child) => child.slug === segment)
      if (!match) break
      path += `/${match.slug}`
      crumbs.push({ title: match.title, path })
      current = match
    }

    return crumbs
  }

  const breadcrumbs = buildBreadcrumbs()

  return (
    <nav className="breadcrumbs-nav flex w-full flex-1 grow text-sm text-gray-500">
      <ol className="flex flex-wrap items-center gap-1">
        {breadcrumbs.map((crumb, index) => (
          <li key={crumb.path} className="flex items-center gap-1">
            <span>/</span>
            {index === breadcrumbs.length - 1 ? (
              <span className="font-semibold text-gray-700">{crumb.title}</span>
            ) : (
              <Link to={crumb.path} className="text-gray-700 hover:underline">
                {crumb.title}
              </Link>
            )}
          </li>
        ))}
      </ol>
    </nav>
  )
}
