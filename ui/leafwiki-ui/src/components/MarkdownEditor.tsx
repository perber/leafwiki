import { remarkLineNumber } from '@/lib/remarkLineNumber'
import { useDebounce } from '@/lib/useDebounce'
import { useCallback, useEffect, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import remarkGfm from 'remark-gfm'
import MarkdownCodeEditor from './MarkdownCodeEditor'
import { MarkdownLink } from './MarkdownLink'

type Props = {
  initialValue?: string
  onChange: (newValue: string) => void
  insert?: string | null
}

export default function MarkdownEditor({
  initialValue = '',
  onChange,
  insert,
}: Props) {
  const previewRef = useRef<HTMLDivElement>(null)
  const rafRef = useRef<number | null>(null)

  const [markdown, setMarkdown] = useState(initialValue)
  const debouncedPreview = useDebounce(markdown, 100)

  const handleEditorChange = useCallback(
    (val: string) => {
      setMarkdown(val)
      onChange(val)
    },
    [onChange],
  )

  const onCursorLineChange = useCallback((line: number) => {
    scrollPreviewToLine(line)
  }, [])

  const scrollPreviewToLine = (line: number) => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current)

    rafRef.current = requestAnimationFrame(() => {
      const preview = previewRef.current
      if (!preview) return

      let target = preview.querySelector(
        `[data-line='${line}']`,
      ) as HTMLElement | null

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
        const table = target.closest('table')
        let offsetTop = 0

        if (table && preview.contains(table)) {
          const previewRect = preview.getBoundingClientRect()
          const targetRect = target.getBoundingClientRect()
          offsetTop = targetRect.top - previewRect.top + preview.scrollTop
        } else {
          offsetTop = target.offsetTop
        }

        const targetHeight = target.offsetHeight
        const containerHeight = preview.clientHeight
        const desiredScrollTop =
          offsetTop - containerHeight / 2 + targetHeight / 2

        const threshold = 16
        const distance = Math.abs(preview.scrollTop - desiredScrollTop)

        if (distance > threshold) {
          preview.scrollTo({ top: desiredScrollTop, behavior: 'smooth' })
        }
      }
    })
  }

  useEffect(() => {
    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
    }
  }, [])

  return (
    <div className="flex h-full gap-1">
      <MarkdownCodeEditor
        initialValue={initialValue}
        onChange={handleEditorChange}
        onCursorLineChange={onCursorLineChange}
        insert={insert}
      />

      <div
        ref={previewRef}
        className="prose prose-lg w-1/2 max-w-none overflow-auto rounded border border-gray-200 bg-white p-4 leading-relaxed [&_img]:h-auto [&_img]:max-w-full [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0"
      >
        <ReactMarkdown
          remarkPlugins={[remarkGfm, remarkLineNumber]}
          rehypePlugins={[rehypeHighlight]}
          components={{
            a: MarkdownLink,
          }}
        >
          {debouncedPreview}
        </ReactMarkdown>
      </div>
    </div>
  )
}
