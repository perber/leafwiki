import { useConfigStore } from '@/stores/config'
import { useTranslation } from 'react-i18next'

type Props = {
  // Direct children of the section, from the client tree. The tree the
  // client holds is already filtered for the viewer (drafts, hidden pages),
  // so counting it never reveals pages the visitor can't see.
  count: number
  testId: string
}

// The number of direct subpages after a section title (tree and section
// overview), shown when the admin turned on Settings > Display > "Show the
// number of subpages". The visible number is hidden from screen readers,
// which get the full sentence instead ("12 subpages").
export function SubpageCount({ count, testId }: Props) {
  const { t } = useTranslation('viewer')
  const enabled = useConfigStore((s) => s.pageDisplay.showSubpageCount)
  if (!enabled) return null
  return (
    <span className="subpage-count" data-testid={testId}>
      <span aria-hidden="true">{count}</span>
      {/* The leading space separates it from the title in the link's
          accessible name ("Guides 12 subpages"). */}
      <span className="sr-only">{` ${t('section.subpageCount', { count })}`}</span>
    </span>
  )
}
