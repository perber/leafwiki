export function Pagination({
  total,
  page,
  limit,
  onPageChange,
}: {
  total: number
  page: number
  limit: number
  onPageChange: (page: number) => void
}) {
  const totalPages = Math.ceil(total / limit)
  if (totalPages <= 1) return null

  return (
    <div className="mt-4 flex items-center justify-center gap-2 text-sm">
      <button
        onClick={() => onPageChange(Math.max(0, page - 1))}
        disabled={page === 0}
        className="rounded border px-2 py-1 disabled:opacity-50"
      >
        ← Prev
      </button>
      <span>
        Page {page + 1} of {totalPages}
      </span>
      <button
        onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
        disabled={page >= totalPages - 1}
        className="rounded border px-2 py-1 disabled:opacity-50"
      >
        Next →
      </button>
    </div>
  )
}
