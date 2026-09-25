import { Button } from '@/components/ui/button'
import i18next from '@/lib/i18n'
import { RefreshCw } from 'lucide-react'
import { memo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { MermaidRenderError } from './useMermaidInjector'
import { useMermaidInjector } from './useMermaidInjector'

export default memo(function MermaidBlock({
  code,
  dataLine,
  theme,
}: {
  code: string
  dataLine?: string
  theme: 'default' | 'dark'
}) {
  const { t } = useTranslation('viewer')
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [error, setError] = useState<MermaidRenderError | null>(null)

  useMermaidInjector({
    containerRef,
    code,
    dataLine,
    theme,
    onError: setError,
  })

  if (error?.isChunkLoadError) {
    return (
      <div
        ref={containerRef}
        className="border-destructive/40 bg-destructive/5 my-4 max-w-full rounded-md border p-4 whitespace-normal"
      >
        <p className="text-destructive text-sm font-medium">
          {t('mermaid.chunkLoadError')}
        </p>
        <Button
          onClick={() => window.location.reload()}
          size="sm"
          className="mt-3 gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          {i18next.t('errorBoundary.reloadButton', { ns: 'common' })}
        </Button>
      </div>
    )
  }

  if (error) {
    return (
      <div
        ref={containerRef}
        className="border-destructive/40 bg-destructive/5 my-4 max-w-full rounded-md border p-4 whitespace-normal"
      >
        <p className="text-destructive text-sm font-medium">
          {t('mermaid.renderError')}
        </p>
        <p className="text-muted-foreground mt-2 pr-12 text-sm break-words">
          {error.message}
        </p>
        <pre className="bg-muted-surface mt-3 overflow-x-auto rounded-md p-3 text-sm">
          <code>{code}</code>
        </pre>
      </div>
    )
  }

  return <div ref={containerRef} className="my-4" />
})
