import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ToolbarButton } from '@/features/toolbar/ToolbarButton'
import { cn } from '@/lib/utils'
import { MoreHorizontal } from 'lucide-react'
import { useToolbarStore } from './toolbar'

const MOBILE_VISIBLE_BUTTONS = 2

export function Toolbar() {
  const buttons = useToolbarStore((state) => state.buttons)
  const overflowButtons = buttons.slice(MOBILE_VISIBLE_BUTTONS)

  return (
    <div className="flex items-center gap-1">
      {buttons.map((button, index) => (
        <ToolbarButton
          key={button.id}
          testId={`${button.id}-button`}
          hotkey={button.hotkey}
          label={button.label}
          onClick={button.action}
          icon={button.icon}
          disabled={button.disabled}
          variant={button.variant}
          className={button.className}
          mobileHidden={index >= MOBILE_VISIBLE_BUTTONS}
        />
      ))}

      {overflowButtons.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="outline"
              size="icon"
              className="h-8 w-8 shadow-xs md:hidden"
              aria-label="More actions"
              data-testid="toolbar-overflow-button"
            >
              <MoreHorizontal size={18} />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52 md:hidden">
            {overflowButtons.map((button) => (
              <DropdownMenuItem
                key={button.id}
                onClick={button.action}
                disabled={button.disabled}
                className={cn(
                  'cursor-pointer',
                  button.className?.includes('text-red-600') && 'text-red-600',
                )}
                data-testid={`${button.id}-menu-item`}
              >
                {button.icon}
                <span>{button.label}</span>
                <DropdownMenuShortcut>{button.hotkey}</DropdownMenuShortcut>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}
