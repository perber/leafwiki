import { useDebounce } from '@/lib/useDebounce'
import { useIsMobile } from '@/lib/useIsMobile'
import { historyField, redo, undo } from '@codemirror/commands'
import { EditorView } from '@codemirror/view'
import { Code2, Eye } from 'lucide-react'
import {
  ClipboardEvent,
  forwardRef,
  JSX,
  MouseEvent as ReactMouseEvent,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import MarkdownPreview from '../preview/MarkdownPreview'
import MarkdownCodeEditor from './MarkdownCodeEditor'
import MarkdownToolbar from './MarkdownToolbar'
import {
  insertHeadingAtStart,
  insertWrappedText,
  replaceFilenameInText,
} from './editorCommands'

import { uploadAsset, UploadAssetResponse } from '@/lib/api/assets'
import { mapApiError } from '@/lib/api/errors'
import { formatBytes, IMAGE_EXTENSIONS } from '@/lib/config'
import { useConfigStore } from '@/stores/config'
import { useEditorStore } from '@/stores/editor'
import { toast } from 'sonner'
import { usePageEditorStore } from './pageEditorStore'
import { slugifyHeadline } from '../preview/rehypeLineNumber'
import { htmlToMarkdown } from './htmlToMarkdown'
import { uploadInlineDataUriImages } from './pasteImageUpload'

export type MarkdownEditorRef = {
  insertAtCursor: (text: string) => void
  getMarkdown: () => string
  insertWrappedText: (before: string, after?: string) => void
  insertHeading: (level: 1 | 2 | 3) => void
  replaceSelection: (text: string) => void
  replaceFilenameInMarkdown?: (before: string, after: string) => void
  editorViewRef: React.RefObject<EditorView | null>
  focus: () => void
  undo: () => void
  redo: () => void
  canUndo: () => boolean
  canRedo: () => boolean
  pasteRich: () => Promise<void>
  pastePlain: () => Promise<void>
}

type Props = {
  initialValue?: string
  onChange: (newValue: string) => void
  pageId: string
}

const DEFAULT_EDITOR_PANE_WIDTH = 50
const MIN_EDITOR_PANE_WIDTH = 25
const MAX_EDITOR_PANE_WIDTH = 75
const EDITOR_PANE_WIDTH_STORAGE_KEY = 'leafwiki-editor-pane-width'

function clampEditorPaneWidth(value: number) {
  return Math.min(MAX_EDITOR_PANE_WIDTH, Math.max(MIN_EDITOR_PANE_WIDTH, value))
}

function getInitialEditorPaneWidth() {
  if (typeof window === 'undefined') return DEFAULT_EDITOR_PANE_WIDTH

  const storedValue = window.localStorage.getItem(EDITOR_PANE_WIDTH_STORAGE_KEY)
  const parsed = Number.parseFloat(storedValue ?? '')

  if (Number.isNaN(parsed)) return DEFAULT_EDITOR_PANE_WIDTH

  return clampEditorPaneWidth(parsed)
}

const MarkdownEditor = (
  { initialValue = '', onChange, pageId }: Props,
  ref: React.ForwardedRef<MarkdownEditorRef>,
) => {
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
    [],
  )

  const { t } = useTranslation('editor')
  const previewRef = useRef<HTMLDivElement | null>(null)
  const desktopSplitRef = useRef<HTMLDivElement | null>(null)

  const setPreviewRef = useCallback((node: HTMLDivElement | null) => {
    if (node) {
      previewRef.current = node
    }
  }, [])
  const path = usePageEditorStore((s) => s.page?.path)
  const editorViewRef = useRef<EditorView | null>(null)
  const rafRef = useRef<number | null>(null)
  const currentCursorLineRef = useRef<number | null>(null)
  const liveEditorPaneWidthRef = useRef(DEFAULT_EDITOR_PANE_WIDTH)
  const resizeHandlersRef = useRef<{
    onMouseMove: (event: MouseEvent) => void
    onMouseUp: () => void
  } | null>(null)
  const [assetVersion, setAssetVersion] = useState(() => Date.now()) // Initial version based on current timestamp

  const [markdown, setMarkdown] = useState(initialValue)
  const [editorPaneWidth, setEditorPaneWidth] = useState(
    getInitialEditorPaneWidth,
  )
  const [isResizingSplit, setIsResizingSplit] = useState(false)
  const debouncedPreview = useDebounce(markdown, 100)
  const isMobile = useIsMobile()
  const maxAssetUploadSizeBytes = useConfigStore(
    (s) => s.maxAssetUploadSizeBytes,
  )

  const {
    previewVisible: showPreview,
    previewStacked,
    togglePreview,
    togglePreviewLayout,
    lineWrap,
  } = useEditorStore()

  const [activeTab, setActiveTab] = useState<'editor' | 'preview'>('editor')

  useEffect(() => {
    liveEditorPaneWidthRef.current = editorPaneWidth
  }, [editorPaneWidth])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem(
      EDITOR_PANE_WIDTH_STORAGE_KEY,
      String(editorPaneWidth),
    )
  }, [editorPaneWidth])

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
        if (file.size > maxAssetUploadSizeBytes) {
          toast.error(
            `File too large. Max ${formatBytes(maxAssetUploadSizeBytes)} allowed.`,
          )
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
          toast.error(mapApiError(err, `Failed to upload ${file.name}`).message)
        }
      }
    },
    [editorViewRef, maxAssetUploadSizeBytes, onChange, pageId, setMarkdown],
  )

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
      insertWrappedText(view, before, after)
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      editorViewRef.current?.focus()
    },
    replaceSelection: (text: string) => {
      const view = editorViewRef.current
      if (!view) return
      const { from, to } = view.state.selection.main
      view.dispatch({
        changes: { from, to, insert: text },
        selection: { anchor: from + text.length },
      })
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

      const updatedText = replaceFilenameInText(docText, before, after)

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
    pasteRich: async () => {
      const startView = editorViewRef.current
      if (!startView) return
      let md: string | null = null
      try {
        if (typeof navigator.clipboard.read === 'function') {
          const items = await navigator.clipboard.read()
          for (const item of items) {
            if (item.types.includes('text/html')) {
              const blob = await item.getType('text/html')
              md = htmlToMarkdown(await blob.text()) || null
              break
            }
          }
          // Reuse the same clipboard read for the plain-text fallback instead
          // of issuing a second navigator.clipboard call, which can trigger a
          // second permission prompt for the same paste action.
          if (!md) {
            for (const item of items) {
              if (item.types.includes('text/plain')) {
                const blob = await item.getType('text/plain')
                md = (await blob.text()) || null
                break
              }
            }
          }
        }
      } catch {
        // clipboard-read permission denied or API unavailable — fall through to readText
      }
      if (!md) {
        try {
          md = (await navigator.clipboard.readText()) || null
        } catch {
          toast.error(t('toolbar.pasteClipboardError'))
          return
        }
      }
      if (!md) return
      md = await uploadInlineDataUriImages(md, pageId, maxAssetUploadSizeBytes)
      // Re-read after awaits: the editor may have been destroyed, or replaced
      // by a different page's editor (navigation during the async clipboard
      // read), while this was pending — only proceed if it's still the same
      // live view the paste was initiated on.
      const view = editorViewRef.current
      if (!view || view !== startView) return
      const sel = view.state.selection.main
      view.dispatch({
        changes: { from: sel.from, to: sel.to, insert: md },
        selection: { anchor: sel.from + md.length },
      })
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      view.focus()
    },
    pastePlain: async () => {
      const startView = editorViewRef.current
      if (!startView) return
      let text: string
      try {
        text = await navigator.clipboard.readText()
      } catch {
        toast.error(t('toolbar.pasteClipboardError'))
        return
      }
      if (!text) return
      // Re-read after await: the editor may have been destroyed, or replaced
      // by a different page's editor (navigation during the async clipboard
      // read), while this was pending — only proceed if it's still the same
      // live view the paste was initiated on.
      const view = editorViewRef.current
      if (!view || view !== startView) return
      const sel = view.state.selection.main
      view.dispatch({
        changes: { from: sel.from, to: sel.to, insert: text },
        selection: { anchor: sel.from + text.length },
      })
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
      view.focus()
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
  }, [assetVersion, debouncedPreview, scrollPreviewToLine, showPreview])

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
  }, [assetVersion, debouncedPreview, scrollPreviewToLine, showPreview])

  useEffect(() => {
    if (!isResizingSplit || !resizeHandlersRef.current) return

    const { onMouseMove, onMouseUp } = resizeHandlersRef.current
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)

    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
  }, [isResizingSplit])

  useEffect(
    () => () => {
      if (!resizeHandlersRef.current) return

      const { onMouseMove, onMouseUp } = resizeHandlersRef.current
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    },
    [],
  )

  const handleSplitResize = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (isMobile || previewStacked || !showPreview) return

    event.preventDefault()
    event.stopPropagation()

    const startX = event.clientX
    const startWidth = editorPaneWidth
    const splitWidth =
      desktopSplitRef.current?.getBoundingClientRect().width ||
      window.innerWidth

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX
      const nextWidth = clampEditorPaneWidth(
        startWidth + (delta / splitWidth) * 100,
      )

      liveEditorPaneWidthRef.current = nextWidth
      setEditorPaneWidth(nextWidth)
    }

    const onMouseUp = () => {
      setEditorPaneWidth(liveEditorPaneWidthRef.current)
      setIsResizingSplit(false)
      resizeHandlersRef.current = null
    }

    resizeHandlersRef.current = { onMouseMove, onMouseUp }
    setIsResizingSplit(true)
  }

  const renderToolbar = useCallback((): JSX.Element => {
    return (
      <MarkdownToolbar
        editorRef={ref as React.RefObject<MarkdownEditorRef>}
        pageId={pageId}
        onTogglePreview={togglePreview}
        onTogglePreviewLayout={togglePreviewLayout}
        previewVisible={showPreview}
        previewStacked={previewStacked}
        onAssetVersionChange={onAssetVersionChange}
      />
    )
  }, [
    onAssetVersionChange,
    pageId,
    ref,
    showPreview,
    previewStacked,
    togglePreview,
    togglePreviewLayout,
  ])

  const renderEditor = useCallback(
    (toolbar: boolean = true): JSX.Element => {
      return (
        <>
          {toolbar && renderToolbar()}
          <MarkdownCodeEditor
            initialValue={markdown}
            resetKey={pageId}
            onChange={handleEditorChange}
            onCursorLineChange={onCursorLineChange}
            editorViewRef={editorViewRef}
            lineWrap={lineWrap}
          />
        </>
      )
    },
    [
      handleEditorChange,
      markdown,
      pageId,
      lineWrap,
      onCursorLineChange,
      renderToolbar,
    ],
  )

  const renderPreview = useCallback((): JSX.Element => {
    return (
      <div
        ref={setPreviewRef}
        className="custom-scrollbar markdown-editor__preview box-content h-full w-full min-w-full overflow-auto"
        id="markdown-preview-container"
      >
        <div className="p-4">
          <MarkdownPreview
            content={debouncedPreview}
            path={path}
            key={assetVersion}
          />
        </div>
      </div>
    )
  }, [assetVersion, debouncedPreview, setPreviewRef, path])

  return (
    <div className="markdown-editor" onPaste={handlePaste}>
      {/* Mobile */}
      {isMobile && (
        <div className="markdown-editor__mobile">
          {/* Mobile Tabs */}
          <div className="markdown-editor__tabs" role="tablist">
            {[
              { id: 'editor', label: 'Editor', icon: <Code2 size={16} /> },
              { id: 'preview', label: 'Preview', icon: <Eye size={16} /> },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as 'editor' | 'preview')}
                className={
                  activeTab === tab.id
                    ? 'markdown-editor__tab-button markdown-editor__tab-button--active'
                    : 'markdown-editor__tab-button markdown-editor__tab-button--inactive'
                }
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </div>

          {activeTab === 'editor' ? renderToolbar() : null}

          <div className="custom-scrollbar markdown-editor__pane-container">
            <div
              className={
                activeTab === 'editor'
                  ? 'markdown-editor__pane'
                  : 'markdown-editor__pane--hidden'
              }
              key="editor"
            >
              {renderEditor(false)}
            </div>
            <div
              className={
                activeTab === 'preview'
                  ? 'markdown-editor__pane'
                  : 'markdown-editor__pane--hidden'
              }
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
          <div
            ref={desktopSplitRef}
            className={
              previewStacked && showPreview
                ? 'markdown-editor__stacked-layout'
                : 'flex w-full flex-1 overflow-hidden'
            }
          >
            <div
              className={
                showPreview
                  ? previewStacked
                    ? 'custom-scrollbar markdown-editor__editor-pane markdown-editor__editor-pane--stacked'
                    : 'custom-scrollbar markdown-editor__editor-pane markdown-editor__editor-pane--half'
                  : 'custom-scrollbar markdown-editor__editor-pane markdown-editor__editor-pane--full'
              }
              style={
                showPreview && !previewStacked
                  ? { flex: `0 0 ${editorPaneWidth}%` }
                  : undefined
              }
            >
              {renderEditor(false)}
            </div>

            {showPreview && (
              <>
                <div
                  className={
                    previewStacked
                      ? 'markdown-editor__divider--stacked'
                      : `markdown-editor__divider ${
                          isResizingSplit
                            ? 'markdown-editor__divider--active'
                            : ''
                        }`
                  }
                  id="editor-preview-divider"
                  onMouseDown={handleSplitResize}
                  role="separator"
                  aria-orientation={previewStacked ? 'horizontal' : 'vertical'}
                  aria-label="Resize editor and preview panes"
                  aria-valuemin={MIN_EDITOR_PANE_WIDTH}
                  aria-valuemax={MAX_EDITOR_PANE_WIDTH}
                  aria-valuenow={
                    previewStacked ? undefined : Math.round(editorPaneWidth)
                  }
                  data-testid="editor-preview-resize-handle"
                />

                <div
                  className={
                    previewStacked
                      ? 'markdown-editor__preview-container markdown-editor__preview-container--stacked'
                      : 'markdown-editor__preview-container'
                  }
                  style={
                    previewStacked ? undefined : { flex: '1 1 0', minWidth: 0 }
                  }
                >
                  {renderPreview()}
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default forwardRef<MarkdownEditorRef, Props>(MarkdownEditor)
