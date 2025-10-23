import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
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
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className={`flex ${parentClassName}`}>{children}</div>
      </TooltipTrigger>
      <TooltipContent
        side={tooltipSide}
        align={tooltipAlign}
        className="bg-gray-700 pt-1 pr-2 pb-1 pl-2"
      >
        {label}
      </TooltipContent>
    </Tooltip>
  )
}
