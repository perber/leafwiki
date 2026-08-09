import copy from 'copy-to-clipboard'
import {
  ReactNode,
  isValidElement,
  useCallback,
  useEffect,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

export function readTextContent(node: ReactNode): string {
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

export function useCodeCopy(code: string) {
  const { t } = useTranslation('viewer')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return

    const timeoutId = window.setTimeout(() => {
      setCopied(false)
    }, 2000)

    return () => {
      window.clearTimeout(timeoutId)
    }
  }, [copied])

  const copyCode = useCallback(() => {
    if (!copy(code)) {
      toast.error(t('codeBlock.copyErrorToast'))
      return
    }

    setCopied(true)
    toast.success(t('codeBlock.copiedToast'))
  }, [code, t])

  return { copied, copyCode }
}
