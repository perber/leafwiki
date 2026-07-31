import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Check, Copy } from 'lucide-react'
import { ClassAttributes, HTMLAttributes, MouseEvent, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { readTextContent, useCodeCopy } from './useCodeCopy'

type Props = ClassAttributes<HTMLElement> &
  HTMLAttributes<HTMLElement> & { children?: ReactNode }

export default function MarkdownInlineCode({
  children,
  className,
  ...codeProps
}: Props) {
  const { t } = useTranslation('viewer')
  const code = readTextContent(children)
  const { copied, copyCode } = useCodeCopy(code)

  const handleCopy = (event: MouseEvent<HTMLButtonElement>) => {
    event.preventDefault()
    event.stopPropagation()
    copyCode()
  }

  return (
    <span className="markdown-inline-code">
      <code {...codeProps} className={`inline-code ${className ?? ''}`.trim()}>
        {children}
      </code>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="markdown-inline-code__copy-button"
            onClick={handleCopy}
            aria-label={
              copied
                ? t('codeBlock.copiedAriaLabel')
                : t('codeBlock.copyAriaLabel')
            }
            data-testid="markdown-inline-code-copy-button"
          >
            {copied ? <Check /> : <Copy />}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="top" align="center">
          {copied ? t('codeBlock.copiedTooltip') : t('codeBlock.copyTooltip')}
        </TooltipContent>
      </Tooltip>
    </span>
  )
}
