import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { Check, Copy } from 'lucide-react'
import {
  ClassAttributes,
  HTMLAttributes,
  ReactNode,
  isValidElement,
} from 'react'
import { useTranslation } from 'react-i18next'
import { splitHighlightedLines } from './splitHighlightedLines'
import { readTextContent, useCodeCopy } from './useCodeCopy'

type CodeElementProps = {
  className?: string
  children?: ReactNode
  'data-line-numbers'?: string | boolean
}

function hasLineNumbers(props: CodeElementProps): boolean {
  const flag = props['data-line-numbers']
  if (flag === true || flag === 'true' || flag === '') {
    return true
  }
  return (props.className ?? '').split(/\s+/).some((cls) => cls.endsWith('='))
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

  const showLineNumbers = hasLineNumbers(child.props)

  const isCodeBlock = className.includes('language-') || code.includes('\n')
  if (!isCodeBlock) {
    return <pre {...preProps}>{children}</pre>
  }

  const highlightedChildren = child.props.children
  const lines = showLineNumbers
    ? splitHighlightedLines(highlightedChildren)
    : null

  return (
    <div
      className={cn(
        'markdown-code-block',
        showLineNumbers && 'markdown-code-block--line-numbers',
      )}
    >
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
        {showLineNumbers && lines ? (
          <code className={className} data-line-numbers="true">
            <span className="markdown-code-block__lines">
              {lines.map((line, index) => (
                <span
                  key={`code-line-${index + 1}`}
                  className="markdown-code-block__line"
                >
                  <span
                    className="markdown-code-block__line-number"
                    aria-hidden="true"
                  >
                    {index + 1}
                  </span>
                  <span className="markdown-code-block__line-content">
                    {line.length > 0 ? line : '\n'}
                  </span>
                </span>
              ))}
            </span>
          </code>
        ) : (
          children
        )}
      </pre>
    </div>
  )
}
