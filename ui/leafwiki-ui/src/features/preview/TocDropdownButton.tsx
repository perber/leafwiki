import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { cn } from '@/lib/utils'
import { ChevronDown } from 'lucide-react'
import { TocEntry } from './extractTocEntries'

type Props = {
  entries: TocEntry[]
  clickable?: boolean
}

function getTocEntryClassName(level: number) {
  if (level <= 1) {
    return 'text-sm font-semibold'
  }

  if (level === 2) {
    return 'pl-6 text-sm font-medium text-foreground/90'
  }

  if (level === 3) {
    return 'pl-10 text-sm text-muted-foreground'
  }

  return 'pl-12 text-sm text-muted-foreground'
}

export function TocDropdownButton({ entries, clickable = true }: Props) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          On this page
          <ChevronDown size={12} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="max-w-96 min-w-48">
        {entries.map((entry) =>
          clickable ? (
            <DropdownMenuItem
              key={entry.id}
              className={cn(
                'cursor-pointer',
                getTocEntryClassName(entry.level),
              )}
              onSelect={() => {
                scrollToHeadlineHash(`#${encodeURIComponent(entry.id)}`, {
                  waitForStableLayout: false,
                })
              }}
            >
              {entry.text}
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem
              key={entry.id}
              className={getTocEntryClassName(entry.level)}
            >
              {entry.text}
            </DropdownMenuItem>
          ),
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
