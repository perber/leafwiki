import { SearchResultItem } from '@/lib/api/search'
import { Link, useLocation } from 'react-router-dom'

export default function SearchResultCard({ item }: { item: SearchResultItem }) {
  const location = useLocation()
  const isActive = location.pathname === `${item.path}`
  return (
    <Link
      to={`${item.path}`}
      className={`block rounded-xl border p-4 shadow-xs transition ${
        isActive
          ? 'border-green-600 bg-green-50'
          : 'border-gray-200 hover:border-green-600 hover:shadow-md'
      }`}
    >
      <div
        className="mb-1 text-lg font-semibold break-words whitespace-normal text-green-700"
        dangerouslySetInnerHTML={{ __html: item.title }}
      />
      <div
        className="mb-2 text-sm break-words whitespace-normal text-gray-600"
        dangerouslySetInnerHTML={{ __html: item.excerpt }}
      />
      <div className="mt-2 text-xs text-gray-400">
        {item.path.split('/').join(' / ')}
      </div>
    </Link>
  )
}
