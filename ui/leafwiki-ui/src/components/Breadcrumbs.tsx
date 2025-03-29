import { useTreeStore } from "@/stores/tree"
import { Link, useLocation } from "react-router-dom"

export default function Breadcrumbs() {
  const { pathname } = useLocation()
  const { tree } = useTreeStore()

  if (!tree) return null

  const segments = pathname.slice(1).split("/").filter(Boolean)

  // Hilfsfunktion zum Titel lookup
  const buildBreadcrumbs = () => {
    const crumbs = []
    let current = tree
    let path = ""
    for (const segment of segments) {
      const match = current.children.find(child => child.slug === segment)
      if (!match) break
      path += `/${match.slug}`
      crumbs.push({ title: match.title, path })
      current = match
    }

    return crumbs
  }

  const breadcrumbs = buildBreadcrumbs()

  return (
    <nav className="text-sm text-gray-500 mb-4">
      <ol className="flex items-center gap-1 flex-wrap">
        {breadcrumbs.map((crumb) => (
          <li key={crumb.path} className="flex items-center gap-1">
            <span>/</span>
            <Link to={crumb.path} className="hover:underline text-gray-700">
              {crumb.title}
            </Link>
          </li>
        ))}
      </ol>
    </nav>
  )
}
