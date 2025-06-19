import { useDebounce } from '@/lib/useDebounce'
import { useIsMobile } from '@/lib/useIsMobile'
import { historyField, redo, undo } from '@codemirror/commands'
import { EditorView } from '@codemirror/view'
import { Code2, Eye } from 'lucide-react'
import {
  forwardRef,
  JSX,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import MarkdownPreview from '../preview/MarkdownPreview'
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
  const previewRef = useRef<HTMLDivElement | null>(null)

  const setPreviewRef = useCallback((node: HTMLDivElement | null) => {
    if (node) {
      previewRef.current = node
    }
  }, [])

  const editorViewRef = useRef<EditorView | null>(null)
  const rafRef = useRef<number | null>(null)
  const [assetVersion, setAssetVersion] = useState(() => Date.now()) // Initial version based on current timestamp

  const [markdown, setMarkdown] = useState(initialValue)
  const debouncedPreview = useDebounce(markdown, 100)
  const isMobile = useIsMobile()

  const [activeTab, setActiveTab] = useState<'editor' | 'preview'>('editor')

  useEffect(() => {
    setMarkdown(initialValue)
  }, [initialValue])

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

  const onAssetVersionChange = useCallback(
    (version: number) => {
      // Update the asset version to trigger a re-render of the preview
      // This is useful when images or other assets change
      // The preview will re-render with the new assets
      setAssetVersion(version)
    },
    [setAssetVersion],
  )

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

  const renderToolbar = useCallback((): JSX.Element => {
    return (
      <MarkdownToolbar
        editorRef={ref as React.RefObject<MarkdownEditorRef>}
        pageId={pageId}
        onAssetVersionChange={onAssetVersionChange}
      />
    )
  }, [onAssetVersionChange, pageId, ref])

  const renderEditor = useCallback(
    (toolbar: boolean = true): JSX.Element => {
      return (
        <>
          {toolbar && renderToolbar()}
          <MarkdownCodeEditor
            initialValue={initialValue}
            onChange={handleEditorChange}
            onCursorLineChange={onCursorLineChange}
            editorViewRef={editorViewRef}
          />
        </>
      )
    },
    [handleEditorChange, initialValue, onCursorLineChange, renderToolbar],
  )

  const renderPreview = useCallback((): JSX.Element => {
    return (
      <div
        ref={setPreviewRef}
        className="prose prose-base box-content h-full w-full overflow-auto"
      >
        <div className="p-4">
          <MarkdownPreview content={debouncedPreview} key={assetVersion} />
        </div>
      </div>
    )
  }, [assetVersion, debouncedPreview, setPreviewRef])

  /*
    Known Issues:
    * When we resize the window, the preview does not update immediately.
    * I will leave the issue open for now. (You can validate this by resizing the window in the edit mode)
  **/

  return (
    <div className="flex h-full w-full flex-col overflow-hidden">
      {/* Mobile */}
      {isMobile && (
        <div className="flex h-full w-full flex-col md:hidden">
          {/* Mobile Tabs */}
          <div className="mb-2 flex border-b text-sm md:hidden" role="tablist">
            {[
              { id: 'editor', label: 'Editor', icon: <Code2 size={16} /> },
              { id: 'preview', label: 'Preview', icon: <Eye size={16} /> },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as 'editor' | 'preview')}
                className={`-mb-px flex flex-1 items-center justify-center gap-1 border-b-2 px-3 py-1.5 ${
                  activeTab === tab.id
                    ? 'border-green-600 font-semibold text-green-600'
                    : 'border-transparent text-gray-500 hover:text-black'
                }`}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </div>
          {activeTab === 'editor' ? renderToolbar() : null}
          <div className="flex max-w-none flex-1 overflow-auto">
            {activeTab === 'editor' ? renderEditor(false) : renderPreview()}
          </div>
        </div>
      )}
      {!isMobile && (
        <div className="flex h-full w-full flex-col max-md:hidden">
          {renderToolbar()}
          <div className="flex w-full flex-1 overflow-hidden">
            <div className="flex w-1/2 max-w-none flex-1 overflow-auto">
              {renderEditor(false)}
            </div>
            <div className="w-1/2 max-w-none flex-1">{renderPreview()}</div>
          </div>
        </div>
      )}
    </div>
  )
}

export default forwardRef<MarkdownEditorRef, Props>(MarkdownEditor)
