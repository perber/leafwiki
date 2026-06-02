import { cn } from '@/lib/utils'
import { TooltipWrapper } from '../../components/TooltipWrapper'
import { Button } from '../../components/ui/button'

export type ToolbarButtonProps = {
  testId?: string
  label: string
  hotkey: string
  variant?: 'outline' | 'ghost' | 'link' | 'destructive' | 'default'
  onClick: () => void
  disabled?: boolean
  active?: boolean
  icon: React.ReactNode
  className?: string
  mobileHidden?: boolean
}

export function ToolbarButton({
  testId,
  label,
  hotkey,
  onClick,
  disabled = false,
  active = false,
  icon,
  variant = 'outline',
  className,
  mobileHidden = false,
}: ToolbarButtonProps) {
  const combinedClassName = cn(
    'h-8 w-8 shadow-xs',
    active &&
      'bg-primary/15 border-primary text-primary hover:bg-primary/20 hover:text-primary',
    mobileHidden && 'hidden md:inline-flex',
    className,
  )

  return (
    <TooltipWrapper
      label={hotkey ? `${label} (${hotkey})` : label}
      side="top"
      align="center"
    >
      <Button
        className={combinedClassName}
        variant={variant}
        size="icon"
        disabled={disabled}
        aria-label={label}
        aria-pressed={active}
        data-testid={
          testId ?? `${label.toLowerCase().replace(/ /g, '-')}-button`
        }
        onClick={onClick}
      >
        {icon}
      </Button>
    </TooltipWrapper>
  )
}
