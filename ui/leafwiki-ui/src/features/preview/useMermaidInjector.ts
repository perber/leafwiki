import mermaid, { RenderResult } from 'mermaid'
import { useEffect, useRef } from 'react'

export type MermaidInjectorOps = {
  containerRef: React.RefObject<HTMLDivElement | null>
  code: string
  dataLine: string
}

function djb2(str: string) {
  let h = 5381
  for (let i = 0; i < str.length; i++) h = ((h << 5) + h) ^ str.charCodeAt(i)
  return (h >>> 0).toString(36)
}

function normalizeCode(code: string) {
  const lines = code.replace(/\r\n/g, '\n').split('\n')
  // remove leading/trailing empty lines
  while (lines.length && lines[0].trim() === '') lines.shift()
  while (lines.length && lines[lines.length - 1].trim() === '') lines.pop()
  // determine common indent
  const indents = lines
    .filter((l) => l.trim() !== '')
    .map((l) => l.match(/^(\s+)/)?.[1].length ?? 0)
  const min = indents.length ? Math.min(...indents) : 0
  // Remove common indent
  const out = lines.map((l) => l.slice(min)).join('\n')
  return out
}

export function useMermaidInjector({
  containerRef,
  code,
  dataLine,
}: MermaidInjectorOps) {
  const lastHashRef = useRef<string | null>(null)
  const lastDataLineRef = useRef<string | null>(null)
  const mermaidInitializedRef = useRef(false)

  // Initialize mermaid only once
  if (!mermaidInitializedRef.current) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'dark',
      deterministicIds: true,
      deterministicIDSeed: 'leafwiki',
    })
    mermaid.setParseErrorHandler((err) => {
      console.warn('Mermaid parse error:', err)
    })
  }
  mermaidInitializedRef.current = true

  useEffect(() => {
    if (!containerRef) return
    if (!containerRef.current) return

    let cancelled = false

    async function inject() {
      const el = containerRef.current
      if (!el) return
      const normalizedCode = normalizeCode(code)

      const codeHash = djb2(normalizedCode)
      if (lastHashRef.current === codeHash) {
        if (lastDataLineRef.current !== dataLine) {
          // Replace data-line of existing SVG in the list of svgs
          lastDataLineRef.current = dataLine
        }
        return // No need to re-render
      }
      // This is required to prevent layout shifts
      const sandbox = document.getElementById('mermaid-renderer')
      if (!sandbox) {
        console.warn('Mermaid renderer element not found')
        return
      }
      await mermaid.parse(normalizedCode)
      const { svg }: RenderResult = await mermaid.render(
        `mermaid-${codeHash}-${dataLine || '0'}`,
        normalizedCode,
        sandbox,
      )

      if (cancelled) return

      const doc = new DOMParser().parseFromString(svg, 'image/svg+xml')
      const newSvg = doc.documentElement as unknown as SVGSVGElement
      newSvg.setAttribute('width', '100%')
      newSvg.removeAttribute('height')
      newSvg.setAttribute('preserveAspectRatio', 'xMinYMin meet')
      if (dataLine != null) newSvg.setAttribute('data-line', String(dataLine))

      const oldSVG = el.querySelector('svg')
      if (oldSVG) {
        el.replaceChild(newSvg, oldSVG)
      } else {
        el.appendChild(newSvg)
      }

      // Update refs
      lastHashRef.current = codeHash
      lastDataLineRef.current = dataLine

      // Add dataLine to Parent container
      if (dataLine != null) {
        el.setAttribute('data-line', String(dataLine))
      } else {
        el.removeAttribute('data-line')
      }

      // Unlock height
      el.style.minHeight = ''
    }

    const raf1 = requestAnimationFrame(inject)
    return () => {
      cancelled = true
      if (raf1) cancelAnimationFrame(raf1)
    }
  }, [containerRef, code, dataLine])
}
