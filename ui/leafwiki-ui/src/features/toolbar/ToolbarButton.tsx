import { cn } from '@/lib/utils'
import { TooltipWrapper } from '../../components/TooltipWrapper'
import { Button } from '../../components/ui/button'

export type ToolbarButtonProps = {
  label: string
  hotkey: string
  variant?: 'outline' | 'ghost' | 'link' | 'destructive' | 'default'
  onClick: () => void
  disabled?: boolean
  icon: React.ReactNode
  className?: string
}

export function ToolbarButton({
  label,
  hotkey,
  onClick,
  disabled = false,
  icon,
  variant = 'outline',
  className,
}: ToolbarButtonProps) {
  const combinedClassName = cn('h-8 w-8 shadow-xs', className)

  return (
    <TooltipWrapper label={`${label} (${hotkey})`} side="top" align="center">
      <Button
        className={combinedClassName}
        variant={variant}
        size="icon"
        disabled={disabled}
        aria-label={label}
        data-testid={`${label.toLowerCase().replace(/ /g, '-')}-button`}
        onClick={onClick}
      >
        {icon}
      </Button>
    </TooltipWrapper>
  )
}
