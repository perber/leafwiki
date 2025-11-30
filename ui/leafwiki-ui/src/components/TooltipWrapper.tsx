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
}

export function TooltipWrapper({
  label,
  children,
  side,
  align,
  parentClassName,
}: Props) {
  const tooltipSide = side || 'top'
  const tooltipAlign = align || 'start'
  const isMobile = useIsMobile()

  if (isMobile) {
    return <>{children}</>
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className={clsx('flex', parentClassName)}>{children}</div>
      </TooltipTrigger>
      <TooltipContent
        side={tooltipSide}
        align={tooltipAlign}
        className="tooltip-wrapper__content"
      >
        {label}
      </TooltipContent>
    </Tooltip>
  )
}
