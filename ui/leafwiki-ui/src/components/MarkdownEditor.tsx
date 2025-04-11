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
      
            // Insert tab or spaces (here we use 2 spaces)
            const newValue =
              value.substring(0, start) + '  ' + value.substring(end)
      
            onChange(newValue)
      
            // Restore cursor position after inserted spaces
            requestAnimationFrame(() => {
              textarea.selectionStart = textarea.selectionEnd = start + 2
            })
          }
        }}
        placeholder="Write in Markdown..."
      />

      <div className="prose prose-lg w-1/2 max-w-none overflow-auto rounded border border-gray-200 bg-white p-4 leading-relaxed [&_li]:leading-snug [&_ol_ol]:mb-0 [&_ol_ol]:mt-0 [&_ol_ul]:mt-0 [&_ul>li::marker]:text-gray-800 [&_ul_ol]:mb-0 [&_ul_ul]:mb-0 [&_ul_ul]:mt-0">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{value}</ReactMarkdown>
      </div>
    </div>
  )
}
