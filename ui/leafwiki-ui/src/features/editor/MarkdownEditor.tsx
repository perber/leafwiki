/* eslint-disable react-hooks/set-state-in-effect */

import { useDebounce } from '@/lib/useDebounce'
import { useIsMobile } from '@/lib/useIsMobile'
import { historyField, redo, undo } from '@codemirror/commands'
import { EditorView } from '@codemirror/view'
import { Code2, Eye } from 'lucide-react'
import {
  ClipboardEvent,
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
import { insertHeadingAtStart, insertWrappedText } from './editorCommands'

import { uploadAsset, UploadAssetResponse } from '@/lib/api/assets'
import {
  IMAGE_EXTENSIONS,
  MAX_UPLOAD_SIZE,
  MAX_UPLOAD_SIZE_MB,
} from '@/lib/config'
import { useEditorStore } from '@/stores/editor'
import { toast } from 'sonner'

export type MarkdownEditorRef = {
  insertAtCursor: (text: string) => void
  getMarkdown: () => string
  insertWrappedText: (before: string, after?: string) => void
  insertHeading: (level: 1 | 2 | 3) => void
  replaceFilenameInMarkdown?: (before: string, after: string) => void
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

  const { previewVisible: showPreview, togglePreview } = useEditorStore()

  const [activeTab, setActiveTab] = useState<'editor' | 'preview'>('editor')

  // Handles paste requests.
  // This allows to paste images from clipboard directly into the editor.
  const handlePaste = useCallback(
    async (event: ClipboardEvent<HTMLDivElement>) => {
      const { clipboardData } = event
      if (!clipboardData) return

      const files: File[] = []

      if (clipboardData.files && clipboardData.files.length > 0) {
        files.push(...Array.from(clipboardData.files))
      } else if (clipboardData.items && clipboardData.items.length > 0) {
        Array.from(clipboardData.items).forEach((item) => {
          if (item.kind === 'file') {
            const file = item.getAsFile()
            if (file) {
              files.push(file)
            }
          }
        })
      }

      if (files.length === 0) {
        return
      }

      // We take over the paste event to handle image files
      event.preventDefault()
      event.stopPropagation()

      // Process each file
      for (const file of files) {
        if (file.size > MAX_UPLOAD_SIZE) {
          toast.error(`File too large. Max ${MAX_UPLOAD_SIZE_MB}MB allowed.`)
          continue
        }

        // Upload each file
        try {
          const res: UploadAssetResponse = await uploadAsset(pageId, file)

          toast.success(`Uploaded ${file.name}`)

          // The result of uploadAsset looks like this:
          // {"file":"/assets/0NmpvSivg/preview-scrollbar.gif"}
          const uploadedFile = res.file
          const ext = file.name.split('.').pop()?.toLowerCase()

          const isImage =
            file.type.startsWith('image/') ||
            IMAGE_EXTENSIONS.includes(ext ?? '')

          const markdown = isImage
            ? `![${file.name}](${uploadedFile})\n`
            : `[${file.name}](${uploadedFile})\n`

          const view = editorViewRef.current
          if (!view) continue
          const { from } = view.state.selection.main
          view.dispatch({
            changes: { from, insert: markdown },
            selection: { anchor: from + markdown.length },
          })

          const newDoc = view.state.doc.toString()
          setMarkdown(newDoc)
          onChange(newDoc)
          editorViewRef.current?.focus()
        } catch (err) {
          console.error('Upload failed', err)
          toast.error(`Failed to upload ${file.name}`)
        }
      }
    },
    [onChange, pageId, setMarkdown, editorViewRef],
  )

  // Set initial markdown value when component mounts
  // This sets the initial value only once
  useEffect(() => {
    setMarkdown((prev) => (prev === '' ? initialValue : prev))
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
      insertWrappedText(view, before, after)
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      editorViewRef.current?.focus()
    },
    insertHeading: (level: 1 | 2 | 3) => {
      const view = editorViewRef.current
      if (!view) return
      insertHeadingAtStart(view, level)
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      editorViewRef.current?.focus()
    },
    replaceFilenameInMarkdown: (before: string, after: string) => {
      const view = editorViewRef.current
      if (!view) return
      const docText = view.state.doc.toString()

      // Just replace the filename.
      // The path remains unchanged, but the filename has to start with '/'
      const regex = new RegExp(`(!?\\[.*?\\]\\(.*?/?)/${before}(\\))`, 'g')

      const newFilename = after.startsWith('/') ? after.slice(1) : after
      const updatedText = docText.replace(regex, `$1/${newFilename}$2`)

      // Replace the entire document content
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: updatedText },
      })

      setMarkdown(updatedText)
      onChange(updatedText)
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

  const onCursorLineChange = useCallback((line: number) => {
    scrollPreviewToLine(line)
  }, [])

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
        onTogglePreview={togglePreview}
        previewVisible={showPreview}
        onAssetVersionChange={onAssetVersionChange}
      />
    )
  }, [onAssetVersionChange, pageId, ref, showPreview, togglePreview])

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
        className="prose prose-base custom-scrollbar box-content h-full w-full overflow-auto"
        id="markdown-preview-container"
      >
        <div className="p-4">
          <MarkdownPreview content={debouncedPreview} key={assetVersion} />
        </div>
      </div>
    )
  }, [assetVersion, debouncedPreview, setPreviewRef])

  // TODO: Known Issues:
  // * When we resize the window, the preview does not update immediately.
  // * I will leave the issue open for now. (You can validate this by resizing the window in the edit mode)

  return (
    <div
      className="flex h-full w-full flex-col overflow-hidden"
      key={isMobile ? 'mobile' : 'desktop'}
      onPaste={handlePaste}
    >
      {/* Mobile */}
      {isMobile && (
        <div className="flex h-full w-full flex-col">
          {/* Mobile Tabs */}
          <div className="mb-2 flex border-b text-sm" role="tablist">
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
          <div className="custom-scrollbar flex max-w-none flex-1 overflow-auto">
            <div
              className={activeTab === 'editor' ? 'block w-full' : 'hidden'}
              key="editor"
            >
              {renderEditor(false)}
            </div>
            <div
              className={activeTab === 'preview' ? 'block w-full' : 'hidden'}
              key="preview"
            >
              {renderPreview()}
            </div>
          </div>
        </div>
      )}
      {!isMobile && (
        <div className="flex h-full w-full flex-col">
          {renderToolbar()}
          <div className="flex w-full flex-1 overflow-hidden">
            <div
              className={`custom-scrollbar flex ${showPreview ? 'w-1/2' : 'w-full'} max-w-none flex-1 overflow-auto`}
            >
              {renderEditor(false)}
            </div>
            {showPreview && (
              <>
                <div
                  className="h-full w-1 bg-gray-300"
                  id="editor-preview-divider"
                ></div>
                <div className="w-1/2 max-w-none flex-1">{renderPreview()}</div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default forwardRef<MarkdownEditorRef, Props>(MarkdownEditor)
