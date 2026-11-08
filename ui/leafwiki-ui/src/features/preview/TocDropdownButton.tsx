import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { ChevronDown } from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import {
  panelTitleKey,
  visiblePanelSections,
  type PagePanelItem,
  type PagePanelSection,
} from './pagePanelSection'

type Props = {
  sections: PagePanelSection[]
}

function getTocEntryClassName(level: number) {
  if (level <= 1) {
    return 'text-sm font-semibold'
  }

  if (level === 2) {
    return 'pl-6 text-sm font-medium text-foreground/90'
  }

  if (level === 3) {
    return 'pl-10 text-sm'
  }

  return 'pl-12 text-sm'
}

export function TocDropdownButton({ sections }: Props) {
  const { t } = useTranslation('viewer')

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          {t(panelTitleKey(sections))}
          <ChevronDown size={12} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="max-h-[70vh] max-w-96 min-w-48 overflow-y-auto"
      >
        {visiblePanelSections(sections).map((section, index) => (
          <Fragment key={section.id}>
            {/* The first section is titled by the trigger button. */}
            {index > 0 && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuLabel>{t(section.titleKey)}</DropdownMenuLabel>
              </>
            )}
            {section.items.map((item) => (
              <DropdownItem key={item.key} item={item} />
            ))}
          </Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function DropdownItem({ item }: { item: PagePanelItem }) {
  if (item.kind === 'download') {
    return (
      <DropdownMenuItem asChild>
        <a
          href={item.href}
          download={item.fileName}
          className="cursor-pointer text-sm"
          title={item.label}
          data-testid={`toc-dropdown-${item.testId}`}
        >
          {item.label}
        </a>
      </DropdownMenuItem>
    )
  }

  return (
    <DropdownMenuItem
      className={cn(
        getTocEntryClassName(item.level ?? 1),
        item.active && 'text-brand',
        item.onSelect && 'cursor-pointer',
      )}
      onSelect={item.onSelect}
      data-testid={`toc-dropdown-${item.testId}`}
    >
      {item.label}
    </DropdownMenuItem>
  )
}
