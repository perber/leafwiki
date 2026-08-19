import { Link2Off, Loader2, RefreshCw, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { fetchBrokenLinks, type BrokenLinksResult } from '@/lib/api/links'
import { createNavigationVisitState } from '@/lib/navigationVisit'

type BrokenLinkReference = {
  from_page_id: string
  from_path: string
  from_title: string
}

type BrokenLinkGroup = {
  to_path: string
  to_title: string
  references: BrokenLinkReference[]
}

export default function BrokenLinks() {
  const { t } = useTranslation('brokenLinks')
  const [data, setData] = useState<BrokenLinksResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (isRefresh = false) => {
    if (isRefresh) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    setError(null)

    try {
      setData(await fetchBrokenLinks())
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t('error.loadFailed'),
      )
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const groups = useMemo<BrokenLinkGroup[]>(() => {
    if (!data) return []

    const grouped = new Map<string, BrokenLinkGroup>()

    for (const link of data.links) {
      const existing = grouped.get(link.to_path)

      const reference = {
        from_page_id: link.from_page_id,
        from_path: link.from_path,
        from_title: link.from_title,
      }

      if (existing) {
        existing.references.push(reference)
        continue
      }

      grouped.set(link.to_path, {
        to_path: link.to_path,
        to_title: link.to_title,
        references: [reference],
      })
    }

    return [...grouped.values()].sort((a, b) =>
      a.to_path.localeCompare(b.to_path),
    )
  }, [data])

  const brokenLinks = data?.links.length ?? 0
  const missingPages = groups.length

  const affectedPages = useMemo(() => {
    if (!data) return 0

    return new Set(data.links.map((link) => link.from_page_id)).size
  }, [data])

  return (
    <div className="w-full">
      <div className="mb-6 flex items-start justify-between gap-6">
        <div>
          <div className="flex items-center gap-3">
            <div className="border-error/20 bg-error/5 text-error grid h-9 w-9 shrink-0 place-items-center rounded-lg border">
              <Link2Off className="h-4.5 w-4.5" />
            </div>

            <h2 className="settings__section-title">{t('title')}</h2>
          </div>

          <p className="settings__section-description">{t('description')}</p>
        </div>

        <Button
          className="settings__actions mb-4"
          onClick={() => load(true)}
          disabled={loading || refreshing}
        >
          <RefreshCw
            className={`mr-2 h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
          />
          {refreshing ? t('button.refreshing') : t('button.text')}
        </Button>
      </div>

      {error && (
        <div className="border-error/20 bg-error/5 text-error mb-6 rounded-lg border px-4 py-3 text-sm">
          {error}
        </div>
      )}

      <div className="mb-6 grid gap-3 sm:grid-cols-3">
        <Stat
          label={t('summaryCard.broken')}
          value={brokenLinks}
          danger
          loading={loading}
        />

        <Stat label={t('summaryCard.missing')} value={missingPages} loading={loading} />

        <Stat label={t('summaryCard.affected')} value={affectedPages} loading={loading} />
      </div>

      <div className="mb-2 flex items-center justify-between">
        <h2 className="settings__section-title text-sm">{t('heading')}</h2>

        <span className="text-muted text-xs">{t('sortLabel')}</span>
      </div>

      {loading ? (
        <div className="border-border text-muted flex items-center justify-center rounded-lg border py-16 text-sm">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Loading broken links…
        </div>
      ) : groups.length === 0 ? (
        <EmptyState />
      ) : (
        <div className="flex flex-col gap-2.5">
          {groups.map((group) => (
            <BrokenLinkGroupCard key={group.to_path} group={group} />
          ))}
        </div>
      )}
    </div>
  )
}

function Stat({
  label,
  value,
  danger = false,
  loading,
}: {
  label: string
  value: number
  danger?: boolean
  loading: boolean
}) {
  return (
    <div className="border-border bg-background rounded-lg border px-4 py-3.5">
      <div className="text-muted mb-1 text-xs">{label}</div>

      {loading ? (
        <Loader2 className="text-muted h-5 w-5 animate-spin" />
      ) : (
        <div
          className={`text-2xl font-semibold ${
            danger ? 'text-error' : 'text-interface-text'
          }`}
        >
          {value}
        </div>
      )}
    </div>
  )
}

function displayMissingPage(path: string): string {
  return path.startsWith('wikilink:') ? path.slice('wikilink:'.length) : path
}

function BrokenLinkGroupCard({ group }: { group: BrokenLinkGroup }) {
  const { t } = useTranslation('brokenLinks')
  
  return (
    <section className="border-border bg-background overflow-hidden rounded-lg border">
      <div className="border-border bg-surface flex items-center gap-2.5 border-b px-4 py-3">
        <TriangleAlert className="text-error h-4 w-4 shrink-0" />

        <span className="text-error min-w-0 truncate font-mono text-[13px] font-semibold">
          {displayMissingPage(group.to_path)}
        </span>

        <span className="bg-error/10 text-error ml-auto shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold">
          {group.references.length}{' '}
          {group.references.length === 1 ? 'reference' : 'references'}
        </span>
      </div>

      <div className="px-4 py-1.5">
        {group.references.map((reference) => (
          <div
            key={`${group.to_path}:${reference.from_page_id}`}
            className="border-border/50 flex items-center gap-3 border-b px-1 py-2.5 last:border-0"
          >
            <div className="bg-surface text-muted grid h-7 w-7 shrink-0 place-items-center rounded-md">
              <Link2Off className="h-3.5 w-3.5" />
            </div>

            <div className="min-w-0 flex-1">
              <Link
                to={reference.from_path}
                state={createNavigationVisitState()}
                className="text-primary text-[13px] font-medium hover:underline"
              >
                {reference.from_title}
              </Link>

              <div className="text-muted mt-0.5 text-[11px]">
                {t('linksTo')} {displayMissingPage(group.to_path)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

function EmptyState() {
  const { t } = useTranslation('brokenLinks')

  return (
    <div className="border-border rounded-lg border border-dashed px-5 py-16 text-center">
      <div className="text-success mb-3 text-3xl">✓</div>

      <h2 className="mb-1 text-base font-semibold">{t('empty.title')}</h2>

      <p className="text-muted text-sm">{t('empty.description')}</p>
    </div>
  )
}
