import { useTranslation } from 'react-i18next'

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
  const { t } = useTranslation('common')
  const totalPages = Math.ceil(total / limit)
  if (totalPages <= 1) return null

  return (
    <div className="pagination">
      <button
        onClick={() => onPageChange(Math.max(0, page - 1))}
        disabled={page === 0}
        className="pagination__button"
      >
        {t('pagination.prev')}
      </button>
      <span>
        {t('pagination.pageOf', { page: page + 1, total: totalPages })}
      </span>
      <button
        onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
        disabled={page >= totalPages - 1}
        className="pagination__button"
      >
        {t('pagination.next')}
      </button>
    </div>
  )
}
