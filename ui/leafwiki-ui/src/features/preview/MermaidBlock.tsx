import { memo, useRef, useState } from 'react'
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
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useMermaidInjector({
    containerRef,
    code,
    dataLine: dataLine as string,
    theme,
    onError: setErrorMessage,
  })

  if (errorMessage) {
    return (
      <div
        ref={containerRef}
        className="border-destructive/40 bg-destructive/5 my-4 rounded-md border p-4"
      >
        <p className="text-destructive text-sm font-medium">
          Unable to render Mermaid diagram.
        </p>
        <p className="text-muted-foreground mt-2 text-sm">{errorMessage}</p>
        <pre className="bg-muted mt-3 overflow-x-auto rounded-md p-3 text-sm">
          <code>{code}</code>
        </pre>
      </div>
    )
  }

  return <div ref={containerRef} className="my-4" />
})
