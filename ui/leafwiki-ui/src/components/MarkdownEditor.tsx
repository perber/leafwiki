import { remarkLineNumber } from '@/lib/remarkLineNumber'
import { useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

export default function MarkdownEditor({
  value = '',
  onChange,
  insert,
}: {
  value?: string
  onChange: (newValue: string) => void
  insert?: string | null
}) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const previewRef = useRef<HTMLDivElement>(null)
  const rafRef = useRef<number | null>(null)

  const handleCursorMove = () => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current)

    rafRef.current = requestAnimationFrame(() => {
      const textarea = textareaRef.current
      const preview = previewRef.current
      if (!textarea || !preview) return

      const textBeforeCursor = textarea.value.slice(0, textarea.selectionStart)
      const line = textBeforeCursor.split('\n').length

      let target = preview.querySelector(
        `[data-line='${line}']`,
      ) as HTMLElement | null

      // Fallback: vorherige existierende data-line suchen
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
        const targetOffset = target.offsetTop
        const targetHeight = target.offsetHeight
        const containerHeight = preview.clientHeight
        const desiredScrollTop =
          targetOffset - containerHeight / 2 + targetHeight / 2

        const threshold = 16 // px Toleranz
        const distance = Math.abs(preview.scrollTop - desiredScrollTop)

        if (distance > threshold) {
          preview.scrollTo({ top: desiredScrollTop, behavior: 'smooth' })
        }
      } else {
        // Fallback: Scroll-Verhältnis synchronisieren
        const canScroll = preview.scrollHeight > preview.clientHeight
        const canScrollTextarea = textarea.scrollHeight > textarea.clientHeight

        if (canScroll && canScrollTextarea) {
          const scrollRatio =
            textarea.scrollTop / (textarea.scrollHeight - textarea.clientHeight)
          const previewTargetScroll =
            scrollRatio * (preview.scrollHeight - preview.clientHeight)

          if (Math.abs(preview.scrollTop - previewTargetScroll) > 16) {
            preview.scrollTo({ top: previewTargetScroll, behavior: 'smooth' })
          }
        }
      }
    })
  }

  useEffect(() => {
    if (insert && textareaRef.current) {
      const textarea = textareaRef.current
      const start = textarea.selectionStart
      const end = textarea.selectionEnd
      const newText = value.slice(0, start) + insert + value.slice(end)

      onChange(newText)

      // Move cursor to end of inserted text
      requestAnimationFrame(() => {
        textarea.selectionStart = textarea.selectionEnd = start + insert.length
        textarea.focus()
      })
    }
  }, [insert])

  return (
    <div className="flex h-full gap-4">
      <textarea
        ref={textareaRef}
        className="w-1/2 resize-none rounded border border-gray-300 p-4 font-mono focus:outline-none focus:ring-2 focus:ring-green-500"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Tab') {
            e.preventDefault()

            const textarea = e.currentTarget
            const start = textarea.selectionStart
            const end = textarea.selectionEnd

            const newValue =
              value.substring(0, start) + '  ' + value.substring(end)

            onChange(newValue)

            requestAnimationFrame(() => {
              textarea.selectionStart = textarea.selectionEnd = start + 2
            })
          }
        }}
        onKeyUp={handleCursorMove}
        onClick={handleCursorMove}
        placeholder="Write in Markdown..."
        spellCheck={false}
      />

      <div
        ref={previewRef}
        className="prose prose-lg w-1/2 max-w-none overflow-auto rounded border border-gray-200 bg-white p-4 leading-relaxed [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0"
      >
        <ReactMarkdown remarkPlugins={[remarkGfm, remarkLineNumber]}>
          {value}
        </ReactMarkdown>
      </div>
    </div>
  )
}
