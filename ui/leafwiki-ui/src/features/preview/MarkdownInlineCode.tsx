import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import copy from 'copy-to-clipboard'
import { Check, Copy } from 'lucide-react'
import { MouseEvent, ReactNode, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type Props = {
  children?: ReactNode
  'data-line'?: string
}

function readTextContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }

  if (Array.isArray(node)) {
    return node.map(readTextContent).join('')
  }

  if (
    typeof node === 'object' &&
    node !== null &&
    'props' in node &&
    typeof (node as { props?: { children?: ReactNode } }).props !== 'undefined'
  ) {
    return readTextContent((node as { props?: { children?: ReactNode } }).props?.children)
  }

  return ''
}

export default function MarkdownInlineCode({ children, 'data-line': dataLine }: Props) {
  const { t } = useTranslation('viewer')
  const [copied, setCopied] = useState(false)

  const value = readTextContent(children).trim()

  useEffect(() => {
    if (!copied) return

    const timeoutId = window.setTimeout(() => {
      setCopied(false)
    }, 1500)

    return () => {
      window.clearTimeout(timeoutId)
    }
  }, [copied])

  const handleCopy = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation()

    const copiedSuccessfully = copy(value)
    if (!copiedSuccessfully) {
      toast.error(t('codeBlock.copyErrorToast'))
      return
    }

    setCopied(true)
    toast.success(t('codeBlock.copiedToast'))
  }

  return (
    <span className="markdown-inline-code">
      <code data-line={dataLine} className="inline-code">
        {children}
      </code>
      <TooltipWrapper
        label={copied ? t('codeBlock.copiedTooltip') : t('codeBlock.copyTooltip')}
      >
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="markdown-inline-code__copy-button"
          onClick={handleCopy}
          aria-label={
            copied ? t('codeBlock.copiedAriaLabel') : t('codeBlock.copyAriaLabel')
          }
          data-testid="markdown-inline-code-copy-button"
        >
          {copied ? <Check /> : <Copy />}
        </Button>
      </TooltipWrapper>
    </span>
  )
}
