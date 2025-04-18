import {
    Tooltip,
    TooltipContent,
    TooltipTrigger
} from '@/components/ui/tooltip'
import { ReactNode } from 'react'

type Props = {
    label: string
    children: ReactNode
    side?: 'left' | 'right' | 'top' | 'bottom'
    align?: 'start' | 'center' | 'end'
}

export function TooltipWrapper({ label, children, side, align }: Props) {
    const tooltipSide = side || 'top'
    const tooltipAlign = align || 'start'
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <div className="flex">{children}</div>
            </TooltipTrigger>
            <TooltipContent side={tooltipSide} align={tooltipAlign} className='bg-gray-700 pl-2 pr-2 pt-1 pb-1'>
                {label}
            </TooltipContent>
        </Tooltip>
    )
}