import { remarkLineNumber } from '@/lib/remarkLineNumber'
import { useDebounce } from '@/lib/useDebounce'
import { historyField, redo, undo } from '@codemirror/commands'
import { EditorView } from '@codemirror/view'
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import ReactMarkdown from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import remarkGfm from 'remark-gfm'
import { MarkdownLink } from '../MarkdownLink'
import MarkdownCodeEditor from './MarkdownCodeEditor'
import MarkdownToolbar from './MarkdownToolbar'

export type MarkdownEditorRef = {
  insertAtCursor: (text: string) => void
  getMarkdown: () => string
  insertWrappedText: (before: string, after?: string) => void
  editorViewRef: React.RefObject<EditorView | null>
  focus: () => void
  undo: () => void
  redo: () => void
  canUndo: () => boolean
  canRedo: () => boolean
}

type Props = {
  initialValue?: string
  onChange: (newValue: string) => void
  pageId: string
}

const MarkdownEditor = (
  { initialValue = '', onChange, pageId }: Props,
  ref: React.ForwardedRef<MarkdownEditorRef>,
) => {
  const previewRef = useRef<HTMLDivElement>(null)
  const editorViewRef = useRef<EditorView | null>(null)
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

  useImperativeHandle(ref, () => ({
    insertAtCursor: (text: string) => {
      const view = editorViewRef.current
      if (!view) return
      const { from } = view.state.selection.main
      view.dispatch({
        changes: { from, insert: text },
        selection: { anchor: from + text.length },
      })
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      editorViewRef.current?.focus()
    },
    insertWrappedText: (before: string, after = before) => {
      const view = editorViewRef.current
      if (!view) return
      const { from, to } = view.state.selection.main
      const hasSelection = from !== to
      const selected = view.state.doc.sliceString(from, to)

      let insertText = ''
      let cursorPos = from

      if (hasSelection) {
        insertText = `${before}${selected}${after}`
        cursorPos = from + insertText.length
      } else {
        insertText = `${before}${after}`
        cursorPos = from + before.length
      }

      view.dispatch({
        changes: { from, to, insert: insertText },
        selection: { anchor: cursorPos },
      })

      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      editorViewRef.current?.focus()
    },
    editorViewRef: editorViewRef,
    getMarkdown: () => editorViewRef.current?.state.doc.toString() || '',
    focus: () => editorViewRef.current?.focus(),
    canUndo: () => {
      const view = editorViewRef.current
      if (!view) return false
      const hist = view.state.field(historyField, false) as
        | {
            done: unknown[]
            undone: unknown[]
          }
        | undefined

      if (!hist || typeof hist !== 'object') return false
      // Not sure why this is > 1, but it seems to work
      // It might be because the initial state counts as a change
      // or because the first change is always recorded in the history

      return hist?.done?.length > 1
    },
    canRedo: () => {
      const view = editorViewRef.current
      if (!view) return false
      const hist = view.state.field(historyField, false) as
        | {
            done: unknown[]
            undone: unknown[]
          }
        | undefined

      if (!hist || typeof hist !== 'object') return false

      return hist?.undone?.length > 0
    },
    undo: () => {
      const view = editorViewRef.current
      if (view) {
        undo(view)
      }
    },
    redo: () => {
      const view = editorViewRef.current
      if (view) {
        redo(view)
      }
    },
  }))

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
    <div className="flex h-full w-full flex-col">
      <MarkdownToolbar
        editorRef={ref as React.RefObject<MarkdownEditorRef>}
        pageId={pageId}
      />
      <div className="flex h-full max-h-full flex-1 overflow-auto">
        <MarkdownCodeEditor
          initialValue={initialValue}
          onChange={handleEditorChange}
          onCursorLineChange={onCursorLineChange}
          editorViewRef={editorViewRef}
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
    </div>
  )
}

export default forwardRef<MarkdownEditorRef, Props>(MarkdownEditor)
