import clsx from 'clsx'
import { TooltipWrapper } from './TooltipWrapper'
import { Button } from './ui/button'

export type PageToolbarButtonProps = {
  label: string
  hotkey: string
  onClick: () => void
  isDestructive?: boolean
  icon: React.ReactNode
}

export function PageToolbarButton({
  label,
  hotkey,
  onClick,
  isDestructive,
  icon,
}: PageToolbarButtonProps) {
  let className: string = 'h-8 w-8 shadow-xs'
  if (isDestructive) {
    className = clsx(
      className,
      'hover:border-red-300 hover:text-red-600 hover:bg-red-100',
    )
  }

  return (
    <TooltipWrapper label={`${label} (${hotkey})`} side="top" align="center">
      <Button
        className={className}
        variant="outline"
        size="icon"
        aria-label={label}
        data-testid={`${label.toLowerCase().replace(/ /g, '-')}-button`}
        onClick={onClick}
      >
        {icon}
      </Button>
    </TooltipWrapper>
  )
}
