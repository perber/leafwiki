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
    <nav className="breadcrumbs-nav">
      <ol className="breadcrumbs-nav__list">
        {breadcrumbs.map((crumb, index) => (
          <li key={crumb.path} className="breadcrumbs-nav__item">
            <span className="breadcrumbs-nav__separator">/</span>
            {index === breadcrumbs.length - 1 ? (
              <span className="breadcrumbs-nav__current">{crumb.title}</span>
            ) : (
              <Link to={crumb.path} className="breadcrumbs-nav__link">
                {crumb.title}
              </Link>
            )}
          </li>
        ))}
      </ol>
    </nav>
  )
}
