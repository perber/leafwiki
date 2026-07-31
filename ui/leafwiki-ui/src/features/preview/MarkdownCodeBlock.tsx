import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { Check, Copy } from 'lucide-react'
import {
  ClassAttributes,
  HTMLAttributes,
  ReactNode,
  isValidElement,
} from 'react'
import { useTranslation } from 'react-i18next'
import { readTextContent, useCodeCopy } from './useCodeCopy'

type CodeElementProps = {
  className?: string
  children?: ReactNode
}

export default function MarkdownCodeBlock(
  props: ClassAttributes<HTMLPreElement> &
    HTMLAttributes<HTMLPreElement> & { children?: ReactNode; node?: unknown },
) {
  const { t } = useTranslation('viewer')
  const { children, node, ...preProps } = props
  void node
  const child = Array.isArray(children) ? children[0] : children
  const childIsCodeElement = isValidElement<CodeElementProps>(child)
  const className = childIsCodeElement ? (child.props.className ?? '') : ''
  const code = childIsCodeElement ? readTextContent(child.props.children) : ''
  const { copied, copyCode } = useCodeCopy(code)

  if (!childIsCodeElement) {
    return <pre {...preProps}>{children}</pre>
  }

  const isCodeBlock = className.includes('language-') || code.includes('\n')
  if (!isCodeBlock) {
    return <pre {...preProps}>{children}</pre>
  }

  return (
    <div className="markdown-code-block">
      <div className="markdown-code-block__actions">
        <TooltipWrapper
          label={
            copied ? t('codeBlock.copiedTooltip') : t('codeBlock.copyTooltip')
          }
        >
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="markdown-code-block__copy-button"
            onClick={copyCode}
            aria-label={
              copied
                ? t('codeBlock.copiedAriaLabel')
                : t('codeBlock.copyAriaLabel')
            }
            data-testid="markdown-code-copy-button"
          >
            {copied ? <Check /> : <Copy />}
          </Button>
        </TooltipWrapper>
      </div>
      <pre
        {...preProps}
        className={`custom-scrollbar ${preProps.className ?? ''}`.trim()}
      >
        {children}
      </pre>
    </div>
  )
}
