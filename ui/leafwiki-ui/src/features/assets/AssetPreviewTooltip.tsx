import { IMAGE_EXTENSIONS } from '@/lib/config'
import { cn } from '@/lib/utils'
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
        className="asset-preview-tooltip__content"
      >
        {isImage ? (
          <img
            src={url}
            alt={name}
            className="asset-preview-tooltip__image"
          />
        ) : (
          <div className="asset-preview-tooltip__file">
            <FileText size={16} /> {name}
          </div>
        )}
      </Tooltip.Content>
    </Tooltip.Root>
  )
}
