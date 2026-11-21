import { Button } from '@/components/ui/button'
import { CircleHelp } from 'lucide-react'
import { Ref } from 'react'
import { useTranslation } from 'react-i18next'

// Only syntax the search backend actually honours (see buildFuzzyQuery and
// searchColumns in internal/search/search_query.go); each key has an
// example and a description under help.<key> in the search namespace.
const SYNTAX_HELP_KEYS = [
  'prefix',
  'allWords',
  'phrase',
  'column',
  'or',
  'not',
  'wildcard',
] as const

type SearchSyntaxHelpButtonProps = {
  open: boolean
  panelId: string
  onToggle: () => void
  ref?: Ref<HTMLButtonElement>
}

// An inline panel toggled by a button rather than a tooltip, so the help is
// reachable by tapping on touch devices. It only closes on request: it is
// reference text people keep open while typing.
export function SearchSyntaxHelpButton({
  open,
  panelId,
  onToggle,
  ref,
}: SearchSyntaxHelpButtonProps) {
  const { t } = useTranslation('search')

  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="icon"
      className="text-muted hover:text-interface-text h-6 w-6 shrink-0 rounded p-1 hover:bg-transparent"
      aria-label={t('help.buttonLabel')}
      aria-expanded={open}
      aria-controls={panelId}
      data-testid="search-syntax-help-button"
      onClick={onToggle}
    >
      <CircleHelp size={16} />
    </Button>
  )
}

type SearchSyntaxHelpPanelProps = {
  id: string
  open: boolean
}

export function SearchSyntaxHelpPanel({
  id,
  open,
}: SearchSyntaxHelpPanelProps) {
  const { t } = useTranslation('search')

  return (
    <div
      id={id}
      hidden={!open}
      // Focusable so a click into the panel keeps focus in the sidebar.
      tabIndex={-1}
      className="search__help-panel"
      data-testid="search-syntax-help"
    >
      <p className="search__help-title">{t('help.title')}</p>
      <dl className="search__help-list">
        {SYNTAX_HELP_KEYS.map((key) => (
          <div key={key} className="search__help-row">
            <dt>
              <code>{t(`help.${key}.example`)}</code>
            </dt>
            <dd>{t(`help.${key}.description`)}</dd>
          </div>
        ))}
      </dl>
    </div>
  )
}
