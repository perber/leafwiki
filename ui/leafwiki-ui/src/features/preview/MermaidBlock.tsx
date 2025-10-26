import { memo, useRef } from 'react'
import { useMermaidInjector } from './useMermaidInjector'

export default memo(function MermaidBlock({
  code,
  dataLine,
}: {
  code: string
  dataLine?: string
}) {
  const containerRef = useRef<HTMLDivElement | null>(null)

  useMermaidInjector({ containerRef, code, dataLine: dataLine as string })

  return <div ref={containerRef} className="my-4" />
})
