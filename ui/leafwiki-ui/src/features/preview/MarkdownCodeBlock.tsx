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
    HTMLAttributes<HTMLPreElement> & { children?: ReactNode },
) {
  const { children, ...preProps } = props
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
      toast.error('Could not copy code')
      return
    }

    setCopied(true)
    toast.success('Code copied')
  }

  return (
    <div className="custom-scrollbar markdown-code-block">
      <div className="markdown-code-block__actions">
        <TooltipWrapper label={copied ? 'Copied' : 'Copy code'}>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="markdown-code-block__copy-button"
            onClick={handleCopy}
            aria-label={copied ? 'Code copied' : 'Copy code'}
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
