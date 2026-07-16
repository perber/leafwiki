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

type CodeElementProps = {
  className?: string
  children?: ReactNode
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

export default function MarkdownCodeBlock(
  props: ClassAttributes<HTMLPreElement> &
    HTMLAttributes<HTMLPreElement> & { children?: ReactNode; node?: unknown },
) {
  const { t } = useTranslation('editor')
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

  const isCodeBlock = className.includes('language-') || code.includes('\n')
  if (!isCodeBlock) {
    return <pre {...preProps}>{children}</pre>
  }

  const handleCopy = () => {
    const copiedSuccessfully = copy(code)
    if (!copiedSuccessfully) {
      toast.error(t('codeBlock.copyFailed'))
      return
    }

    setCopied(true)
    toast.success(t('codeBlock.copied'))
  }

  const copyLabel = copied ? t('codeBlock.copiedLabel') : t('codeBlock.copyCode')

  return (
    <div className="markdown-code-block">
      <div className="markdown-code-block__actions">
        <TooltipWrapper label={copyLabel}>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="markdown-code-block__copy-button"
            onClick={handleCopy}
            aria-label={copyLabel}
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
