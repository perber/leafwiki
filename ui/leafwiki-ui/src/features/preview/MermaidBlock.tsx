import { memo, useRef } from 'react'
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

  useMermaidInjector({
    containerRef,
    code,
    dataLine: dataLine as string,
    theme,
  })

  return <div ref={containerRef} className="my-4" />
})
