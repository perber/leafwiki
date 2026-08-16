import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/lib/useIsMobile'
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
  const isMobile = useIsMobile()
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
      {!isMobile && (
        <TooltipWrapper
          label={
            copied ? t('codeBlock.copiedTooltip') : t('codeBlock.copyTooltip')
          }
          asChild
        >
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
        </TooltipWrapper>
      )}
    </span>
  )
}
