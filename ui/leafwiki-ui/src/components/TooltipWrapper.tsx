import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useIsMobile } from '@/lib/useIsMobile'
import clsx from 'clsx'
import { ReactNode } from 'react'

type Props = {
  label: string
  children: ReactNode
  parentClassName?: string
  side?: 'left' | 'right' | 'top' | 'bottom'
  align?: 'start' | 'center' | 'end'
  asChild?: boolean
}

export function TooltipWrapper({
  label,
  children,
  side,
  align,
  parentClassName,
  asChild = false,
}: Props) {
  const tooltipSide = side || 'top'
  const tooltipAlign = align || 'start'
  const isMobile = useIsMobile()

  if (isMobile) {
    return asChild ? (
      <>{children}</>
    ) : (
      <div className={clsx('flex', parentClassName)}>{children}</div>
    )
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {asChild ? (
          children
        ) : (
          <div className={clsx('flex', parentClassName)}>{children}</div>
        )}
      </TooltipTrigger>
      <TooltipContent
        side={tooltipSide}
        align={tooltipAlign}
        className={clsx(
          'tooltip-wrapper__content',
          'z-30 rounded-sm border px-2 py-1 text-xs shadow-sm',
          'bg-tooltip border-tooltip-border text-tooltip-text',
        )}
      >
        {label}
      </TooltipContent>
    </Tooltip>
  )
}
