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

export function TocDropdownButton({ entries, clickable = true }: Props) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="h-7 gap-1 text-xs font-medium"
        >
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
                'cursor-pointer text-sm',
                entry.level === 2 && 'pl-6',
                entry.level === 3 && 'pl-10',
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
              className={cn(
                'text-sm',
                entry.level === 2 && 'pl-6',
                entry.level === 3 && 'pl-10',
              )}
            >
              {entry.text}
            </DropdownMenuItem>
          ),
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
