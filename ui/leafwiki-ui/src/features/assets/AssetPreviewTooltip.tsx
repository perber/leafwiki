import { IMAGE_EXTENSIONS } from '@/lib/config'
import { cn } from '@/lib/utils' // If you have clsx or cn helper
import * as Tooltip from '@radix-ui/react-tooltip'
import { FileText } from 'lucide-react'

type Props = {
  url: string
  name: string
  children: React.ReactNode
  className?: string
}

const imageExtensions = IMAGE_EXTENSIONS

export function AssetPreviewTooltip({ url, name, children, className }: Props) {
  const ext = url.split('.').pop()?.toLowerCase() ?? ''
  const isImage = imageExtensions.includes(ext)

  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild>
        <div className={cn('inline-block', className)}>{children}</div>
      </Tooltip.Trigger>
      <Tooltip.Content
        side="right"
        align="center"
        className="z-50 max-w-sm rounded border bg-white p-2 shadow-lg"
      >
        {isImage ? (
          <img
            src={url}
            alt={name}
            className="max-h-[300px] max-w-[300px] rounded border"
          />
        ) : (
          <div className="flex items-center gap-2 text-sm text-gray-700">
            <FileText size={16} /> {name}
          </div>
        )}
      </Tooltip.Content>
    </Tooltip.Root>
  )
}
