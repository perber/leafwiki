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
import { useTranslation } from 'react-i18next'
import MarkdownPreview from '../preview/MarkdownPreview'
import MarkdownCodeEditor from './MarkdownCodeEditor'
import MarkdownToolbar from './MarkdownToolbar'
import {
  insertHeadingAtStart,
  insertWrappedText,
  replaceFilenameInText,
} from './editorCommands'

import { useConfigStore } from '@/stores/config'
import { useEditorStore } from '@/stores/editor'
import { toast } from 'sonner'
import { usePageEditorStore } from './pageEditorStore'
import { useEditorFileUpload } from './useEditorFileUpload'
import { usePreviewScrollSync } from './usePreviewScrollSync'
import {
  MAX_EDITOR_PANE_WIDTH,
  MIN_EDITOR_PANE_WIDTH,
  useSplitPaneResize,
} from './useSplitPaneResize'
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

const MarkdownEditor = (
  { initialValue = '', onChange, pageId }: Props,
  ref: React.ForwardedRef<MarkdownEditorRef>,
) => {
  const { t } = useTranslation('editor')

  const path = usePageEditorStore((s) => s.page?.path)
  const editorViewRef = useRef<EditorView | null>(null)
  const [assetVersion, setAssetVersion] = useState(() => Date.now()) // Initial version based on current timestamp

  const [markdown, setMarkdown] = useState(initialValue)
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

  const {
    desktopSplitRef,
    editorPaneWidth,
    isResizingSplit,
    handleSplitResize,
  } = useSplitPaneResize({ isMobile, showPreview, previewStacked })

  const { setPreviewRef, onCursorLineChange } = usePreviewScrollSync({
    editorViewRef,
    showPreview,
    previewContent: debouncedPreview,
    assetVersion,
  })

  // syncFromView pushes the view's document into local state and to the
  // parent — the step every programmatic edit below ends with.
  const syncFromView = useCallback(
    (view: EditorView) => {
      const newDoc = view.state.doc.toString()
      setMarkdown(newDoc)
      onChange(newDoc)
    },
    [onChange],
  )

  const { handlePaste, handleDrop } = useEditorFileUpload({
    editorViewRef,
    pageId,
    onDocChange: syncFromView,
  })

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

  // Rich paste (HTML → Markdown) is only reachable via Ctrl/Cmd+Shift+V and the
  // toolbar/dropdown buttons for now — plain Ctrl/Cmd+V stays default browser
  // paste while rich paste is being tested. See MarkdownCodeEditor's
  // Shift-Mod-v keymap, which calls this same callback.
  const pasteRich = useCallback(async () => {
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
    syncFromView(view)
    view.focus()
  }, [maxAssetUploadSizeBytes, pageId, syncFromView, t])

  const pastePlain = useCallback(async () => {
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
    syncFromView(view)
    view.focus()
  }, [syncFromView, t])

  useImperativeHandle(ref, () => ({
    insertAtCursor: (text: string) => {
      const view = editorViewRef.current
      if (!view) return
      const { from } = view.state.selection.main
      view.dispatch({
        changes: { from, insert: text },
        selection: { anchor: from + text.length },
      })
      syncFromView(view)
      editorViewRef.current?.focus()
    },
    insertWrappedText: (before: string, after = before) => {
      const view = editorViewRef.current
      if (!view) return
      insertWrappedText(view, before, after)
      syncFromView(view)
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
      syncFromView(view)
      editorViewRef.current?.focus()
    },
    insertHeading: (level: 1 | 2 | 3) => {
      const view = editorViewRef.current
      if (!view) return
      insertHeadingAtStart(view, level)
      syncFromView(view)
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
    pasteRich,
    pastePlain,
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
            onPasteRich={pasteRich}
          />
        </>
      )
    },
    [
      handleEditorChange,
      markdown,
      pageId,
      pasteRich,
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
    <div className="markdown-editor" onPaste={handlePaste} onDrop={handleDrop}>
      {/* Mobile */}
      {isMobile && (
        <div className="markdown-editor__mobile">
          {/* Mobile Tabs */}
          <div className="markdown-editor__tabs" role="tablist">
            {[
              {
                id: 'editor',
                label: t('markdownEditor.editorTab'),
                icon: <Code2 size={16} />,
              },
              {
                id: 'preview',
                label: t('markdownEditor.previewTab'),
                icon: <Eye size={16} />,
              },
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
                showPreview ? { flex: `0 0 ${editorPaneWidth}%` } : undefined
              }
            >
              {renderEditor(false)}
            </div>

            {showPreview && (
              <>
                <div
                  className={
                    previewStacked
                      ? `markdown-editor__divider markdown-editor__divider--stacked ${
                          isResizingSplit
                            ? 'markdown-editor__divider--active'
                            : ''
                        }`
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
                  aria-label={t('markdownEditor.resizeAriaLabel')}
                  aria-valuemin={MIN_EDITOR_PANE_WIDTH}
                  aria-valuemax={MAX_EDITOR_PANE_WIDTH}
                  aria-valuenow={Math.round(editorPaneWidth)}
                  data-testid="editor-preview-resize-handle"
                />

                <div
                  className={
                    previewStacked
                      ? 'markdown-editor__preview-container markdown-editor__preview-container--stacked'
                      : 'markdown-editor__preview-container'
                  }
                  style={
                    previewStacked
                      ? { flex: '1 1 0', minHeight: 0 }
                      : { flex: '1 1 0', minWidth: 0 }
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
