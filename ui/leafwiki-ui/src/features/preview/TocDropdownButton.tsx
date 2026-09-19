import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { PageAttachment } from '@/lib/api/assets'
import { withBasePath } from '@/lib/routePath'
import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { cn } from '@/lib/utils'
import { ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { TocEntry } from './extractTocEntries'
import { useTocScrollSpy } from './useTocScrollSpy'

type Props = {
  entries: TocEntry[]
  clickable?: boolean
  activeId?: string | null
  downloads?: PageAttachment[]
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

export function TocDropdownButton({
  entries,
  clickable = true,
  activeId: externalActiveId,
  downloads = [],
}: Props) {
  const { t } = useTranslation('viewer')
  // When the parent provides activeId, skip internal scroll spy (no duplicate listener).
  const internalActiveId = useTocScrollSpy(
    externalActiveId !== undefined ? [] : entries,
  )
  const activeId =
    externalActiveId !== undefined ? externalActiveId : internalActiveId

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          {t('toc.onThisPage')}
          <ChevronDown size={12} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="max-h-[70vh] max-w-96 min-w-48 overflow-y-auto"
      >
        {entries.map((entry) => (
          <DropdownMenuItem
            key={entry.id}
            className={cn(
              getTocEntryClassName(entry.level),
              activeId === entry.id && 'text-brand',
              clickable && 'cursor-pointer',
            )}
            onSelect={
              clickable
                ? () => {
                    scrollToHeadlineHash(`#${encodeURIComponent(entry.id)}`, {
                      waitForStableLayout: false,
                    })
                  }
                : undefined
            }
          >
            {entry.text}
          </DropdownMenuItem>
        ))}
        {downloads.length > 0 && (
          <>
            {entries.length > 0 && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuLabel>{t('toc.downloads')}</DropdownMenuLabel>
              </>
            )}
            {downloads.map((file) => (
              <DropdownMenuItem key={file.url} asChild>
                <a
                  href={withBasePath(file.url)}
                  download={file.name}
                  className="cursor-pointer text-sm"
                  title={file.name}
                  data-testid={`toc-dropdown-download-${file.name}`}
                >
                  {file.name}
                </a>
              </DropdownMenuItem>
            ))}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
