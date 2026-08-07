import { TooltipWrapper } from '@/components/TooltipWrapper'
import { Button } from '@/components/ui/button'
import copy from 'copy-to-clipboard'
import { Check, Copy } from 'lucide-react'
import {
  ClassAttributes,
  HTMLAttributes,
  ReactNode,
  isValidElement,
  useEffect,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { splitHighlightedLines } from './splitHighlightedLines'

type CodeElementProps = {
  className?: string
  children?: ReactNode
  'data-line-numbers'?: string | boolean
}

function readTextContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }

  if (Array.isArray(node)) {
    return node.map(readTextContent).join('')
  }

  if (isValidElement<{ children?: ReactNode }>(node)) {
    return readTextContent(node.props.children)
  }

  return ''
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
  const [copied, setCopied] = useState(false)
  const child = Array.isArray(children) ? children[0] : children

  useEffect(() => {
    if (!copied) return

    const timeoutId = window.setTimeout(() => {
      setCopied(false)
    }, 2000)

    return () => {
      window.clearTimeout(timeoutId)
    }
  }, [copied])

  if (!isValidElement<CodeElementProps>(child)) {
    return <pre {...preProps}>{children}</pre>
  }

  const className = child.props.className ?? ''
  const code = readTextContent(child.props.children)
  const showLineNumbers = hasLineNumbers(child.props)

  const isCodeBlock = className.includes('language-') || code.includes('\n')
  if (!isCodeBlock) {
    return <pre {...preProps}>{children}</pre>
  }

  const handleCopy = () => {
    const copiedSuccessfully = copy(code)
    if (!copiedSuccessfully) {
      toast.error(t('codeBlock.copyErrorToast'))
      return
    }

    setCopied(true)
    toast.success(t('codeBlock.copiedToast'))
  }

  const highlightedChildren = child.props.children
  const lines = showLineNumbers
    ? splitHighlightedLines(highlightedChildren)
    : null

  return (
    <div
      className={`markdown-code-block${showLineNumbers ? ' markdown-code-block--line-numbers' : ''}`}
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
            onClick={handleCopy}
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
