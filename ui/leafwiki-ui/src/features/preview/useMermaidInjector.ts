import { isDynamicImportChunkError } from '@/lib/dynamicImportError'
import i18next from '@/lib/i18n'
import { useEffect, useRef } from 'react'

type MermaidDefault = (typeof import('mermaid'))['default']

let mermaidPromise: Promise<MermaidDefault> | null = null

function loadMermaid(): Promise<MermaidDefault> {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then((m) => m.default)
  }
  return mermaidPromise
}

// Applied by mermaid to the HTML of every label (on top of 'antiscript').
// Labels are author-controlled and end up in the live page through a
// foreignObject, so they may format text and link, but not draw form controls
// or escape their box: inline style and class (which would reach the app's
// Tailwind utilities) are dropped, and so are dialogs and popovers, which the
// browser draws above the whole page. Mermaid's own wrapper markup, classDef and
// style statements are added outside this pass and keep working. Setting this
// replaces mermaid's default, which forbids only <style>, so keep it here.
const labelSanitizeConfig = {
  FORBID_TAGS: [
    'form',
    'input',
    'button',
    'textarea',
    'select',
    'option',
    'style',
    'iframe',
    'dialog',
  ],
  FORBID_ATTR: [
    'style',
    'class',
    'popover',
    'popovertarget',
    'popovertargetaction',
    'commandfor',
    'command',
    // DOMPurify drops only names that already exist on document; any other
    // name on an img would still define a new document property.
    'name',
  ],
}

let initializedTheme: 'default' | 'dark' | null = null

async function ensureMermaidInitialized(
  theme: 'default' | 'dark',
): Promise<MermaidDefault> {
  const mermaid = await loadMermaid()

  if (initializedTheme !== theme) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'antiscript',
      dompurifyConfig: labelSanitizeConfig,
      theme,
      deterministicIds: true,
      deterministicIDSeed: 'leafwiki',
    })
    mermaid.setParseErrorHandler((err) => {
      console.warn('Mermaid parse error:', err)
    })
    initializedTheme = theme
  }

  return mermaid
}

export type MermaidRenderError = {
  message: string
  isChunkLoadError: boolean
}

export type MermaidInjectorOps = {
  containerRef: React.RefObject<HTMLDivElement | null>
  // Element mermaid renders into to measure labels, owned by MarkdownPreview.
  // Passed by reference: an id lookup could find an element of the rendered
  // page instead, since content may use any id.
  sandboxRef: React.RefObject<HTMLDivElement | null>
  code: string
  dataLine?: string
  theme: 'default' | 'dark'
  onError: (error: MermaidRenderError | null) => void
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
  sandboxRef,
  code,
  dataLine,
  theme,
  onError,
}: MermaidInjectorOps) {
  const lastHashRef = useRef<string | null>(null)
  const lastDataLineRef = useRef<string | undefined>(undefined)

  useEffect(() => {
    if (!containerRef) return
    if (!containerRef.current) return

    let cancelled = false

    async function inject() {
      const el = containerRef.current
      if (!el) return
      const normalizedCode = normalizeCode(code)

      // theme is folded into the cache key (not just normalizedCode) so a
      // theme-only change naturally misses the cache and falls through to a
      // real re-render below — no separate "invalidate on theme change"
      // effect needed to force that.
      const codeHash = djb2(`${normalizedCode}|${theme}`)
      if (lastHashRef.current === codeHash) {
        if (lastDataLineRef.current !== dataLine) {
          // Replace data-line of existing SVG in the list of svgs
          const oldSVG = el.querySelector('svg')
          if (oldSVG) {
            if (dataLine != null) {
              oldSVG.setAttribute('data-line', String(dataLine))
            } else {
              oldSVG.removeAttribute('data-line')
            }
          }
          lastDataLineRef.current = dataLine
        }
        onError(null)
        return // No need to re-render
      }
      // This is required to prevent layout shifts
      const sandbox = sandboxRef.current
      if (!sandbox) {
        console.warn('Mermaid renderer element not found')
        return
      }
      try {
        const mermaid = await ensureMermaidInitialized(theme)
        await mermaid.parse(normalizedCode)
        const { svg } = await mermaid.render(
          `mermaid-${codeHash}-${dataLine || '0'}`,
          normalizedCode,
          sandbox,
        )

        if (cancelled) return

        // Parse via innerHTML (HTML parser) instead of DOMParser with
        // image/svg+xml so that HTML void elements such as <img> inside
        // <foreignObject> nodes do not cause a parseerror element to be
        // injected into the DOM (raw XML error visible to the user).
        // Use <template> to keep the fragment inert (no side-effect fetches).
        // XSS: the SVG comes from mermaid.render with securityLevel 'antiscript',
        // which runs HTML labels through DOMPurify with labelSanitizeConfig.
        // Keep that level (or stricter) and that config when changing the
        // mermaid config.
        const template = document.createElement('template')
        // eslint-disable-next-line no-restricted-syntax
        template.innerHTML = svg
        const newSvg = template.content.querySelector(
          'svg',
        ) as SVGSVGElement | null
        if (!newSvg)
          throw new Error(i18next.t('mermaid.noSvgOutput', { ns: 'viewer' }))
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
        onError(null)
      } catch (error) {
        if (cancelled) return

        lastHashRef.current = null
        const message = error instanceof Error ? error.message : String(error)
        console.warn('Mermaid diagram could not be rendered:', message)
        onError({
          message,
          isChunkLoadError: isDynamicImportChunkError(message),
        })
      }
    }

    const raf1 = requestAnimationFrame(inject)
    return () => {
      cancelled = true
      if (raf1) cancelAnimationFrame(raf1)
    }
  }, [containerRef, sandboxRef, code, dataLine, theme, onError])
}
