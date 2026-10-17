import { EditorView } from '@codemirror/view'
import { RefObject, useCallback, useEffect, useRef } from 'react'
import { slugifyHeadline } from '../preview/rehypeLineNumber'

type Options = {
  editorViewRef: RefObject<EditorView | null>
  showPreview: boolean
  // The rendered preview inputs; the preview is re-aligned to the cursor
  // line whenever either changes.
  previewContent: string
  assetVersion: number
}

// usePreviewScrollSync keeps the preview scrolled to the block that renders
// the editor's current cursor line.
export function usePreviewScrollSync({
  editorViewRef,
  showPreview,
  previewContent,
  assetVersion,
}: Options) {
  const previewRef = useRef<HTMLDivElement | null>(null)
  const rafRef = useRef<number | null>(null)
  const currentCursorLineRef = useRef<number | null>(null)

  const setPreviewRef = useCallback((node: HTMLDivElement | null) => {
    if (node) {
      previewRef.current = node
    }
  }, [])

  const findHeadingTarget = useCallback(
    (preview: HTMLDivElement, line: number): HTMLElement | null => {
      const headingByLine = preview.querySelector(
        `h1[data-line='${line}'], h2[data-line='${line}'], h3[data-line='${line}'], h4[data-line='${line}'], h5[data-line='${line}'], h6[data-line='${line}']`,
      ) as HTMLElement | null
      if (headingByLine) return headingByLine

      const lineText = editorViewRef.current?.state.doc.line(line).text ?? ''
      const headingMatch = lineText.match(/^\s{0,3}(#{1,6})\s+(.*)$/)
      if (!headingMatch) return null

      const headingText = headingMatch[2].replace(/<[^>]*>/g, ' ').trim()
      const headingId = slugifyHeadline(headingText)
      if (!headingId) return null

      const escapedId =
        typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
          ? CSS.escape(headingId)
          : headingId

      return preview.querySelector(`#${escapedId}`) as HTMLElement | null
    },
    [editorViewRef],
  )

  const scrollPreviewToLine = useCallback(
    (line: number, behavior: ScrollBehavior = 'smooth') => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current)

      rafRef.current = requestAnimationFrame(() => {
        const preview = previewRef.current
        if (!preview) return

        let target =
          findHeadingTarget(preview, line) ??
          (preview.querySelector(`[data-line='${line}']`) as HTMLElement | null)

        if (!target) {
          for (let i = line - 1; i > 0; i--) {
            const fallback = preview.querySelector(
              `[data-line='${i}']`,
            ) as HTMLElement | null
            if (fallback) {
              target = fallback
              break
            }
          }
        }

        if (target) {
          // Measure relative to the preview viewport instead of relying on
          // offsetTop, which can be relative to an intermediate offsetParent.
          const previewRect = preview.getBoundingClientRect()
          const targetRect = target.getBoundingClientRect()
          const offsetTop = targetRect.top - previewRect.top + preview.scrollTop

          const targetHeight = target.offsetHeight
          const containerHeight = preview.clientHeight
          const maxScrollTop = Math.max(
            0,
            preview.scrollHeight - containerHeight,
          )
          const desiredScrollTop = Math.max(
            0,
            Math.min(
              maxScrollTop,
              offsetTop - containerHeight / 2 + targetHeight / 2,
            ),
          )

          const threshold = 16
          const distance = Math.abs(preview.scrollTop - desiredScrollTop)

          if (distance > threshold) {
            preview.scrollTo({ top: desiredScrollTop, behavior })
          }
        }
      })
    },
    [findHeadingTarget],
  )

  const onCursorLineChange = useCallback(
    (line: number) => {
      currentCursorLineRef.current = line
      scrollPreviewToLine(line, 'smooth')
    },
    [scrollPreviewToLine],
  )

  useEffect(() => {
    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
    }
  }, [])

  useEffect(() => {
    if (!showPreview) return
    const currentLine = currentCursorLineRef.current
    if (currentLine == null) return

    scrollPreviewToLine(currentLine, 'auto')
  }, [assetVersion, previewContent, scrollPreviewToLine, showPreview])

  useEffect(() => {
    if (!showPreview) return

    const preview = previewRef.current
    const content = preview?.firstElementChild
    if (!preview || !content || typeof ResizeObserver === 'undefined') return

    const observer = new ResizeObserver(() => {
      const currentLine = currentCursorLineRef.current
      if (currentLine == null) return

      scrollPreviewToLine(currentLine, 'auto')
    })

    observer.observe(content)

    return () => {
      observer.disconnect()
    }
  }, [assetVersion, previewContent, scrollPreviewToLine, showPreview])

  return { setPreviewRef, onCursorLineChange }
}
