import Page404 from '@/components/Page404'
import { mapApiError, asApiLocalizedError } from '@/lib/api/errors'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { buildBrowserEditUrl } from '@/lib/routePath'
import { DIALOG_LINK_INSERT } from '@/lib/registries'
import { getWikiTargetRoutePath } from '@/lib/wikiPath'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useRef } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import MarkdownEditor, { MarkdownEditorRef } from './MarkdownEditor'
import { PageFrontmatterPanel } from './PageFrontmatterPanel'
import { usePageEditorStore } from './pageEditorStore'
import { isDirtyState } from './pageEditorStore'
import { useAutoSave } from './useAutoSave'
import useNavigationGuard from './useNavigationGuard'
import { useToolbarActions } from './useToolbarActions'

export default function PageEditor() {
  const { '*': path } = useParams()

  const { pathname } = useLocation()
  const navigate = useNavigate()
  const editorRef = useRef<MarkdownEditorRef>(null)
  const skipNavigationGuardRef = useRef(false)
  const openDialog = useDialogsStore((s) => s.openDialog)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const savePage = usePageEditorStore((s) => s.savePage)
  const forceOverwrite = usePageEditorStore((s) => s.forceOverwrite)
  const setContent = usePageEditorStore((s) => s.setContent)
  const setTags = usePageEditorStore((s) => s.setTags)
  const setFrontmatterFields = usePageEditorStore((s) => s.setFrontmatterFields)
  const loadPageData = usePageEditorStore((s) => s.loadPageData)
  const initialPage = usePageEditorStore((s) => s.initialPage) // contains the initial page data when loaded
  const tags = usePageEditorStore((s) => s.tags)
  const frontmatterFields = usePageEditorStore((s) => s.frontmatterFields)
  const frontmatterUnsupported = usePageEditorStore(
    (s) => s.frontmatterUnsupported,
  )
  const frontmatterErrors = usePageEditorStore((s) => s.frontmatterErrors)
  const notFound = usePageEditorStore((s) => s.notFound)
  const error = usePageEditorStore((s) => s.error)
  const openNode = useTreeStore((s) => s.openNode)
  const dirty = usePageEditorStore(isDirtyState)

  // Auto-save hook — must be called unconditionally
  useAutoSave()

  // Shows Unsaved Changes Dialog when navigating away with dirty state
  useNavigationGuard({
    when: () => dirty && !skipNavigationGuardRef.current,
    onNavigate: async () => {
      await reloadTree()
    },
  })

  // Load page data when path changes
  useEffect(() => {
    if (!path) return
    loadPageData(path)
  }, [path, loadPageData])

  // Open node
  useEffect(() => {
    if (!initialPage?.id) return
    openNode(initialPage.id)
  }, [openNode, initialPage?.id])

  // callbacks to save / close
  const handleSave = useCallback(() => {
    savePage()
      .then(async (page) => {
        if (page) {
          window.history.replaceState(
            null,
            '',
            buildBrowserEditUrl(`/${page?.path}`),
          )
          toast.success('Page saved successfully')
        }
      })
      .catch((err) => {
        const localized = asApiLocalizedError(err)
        if (localized?.code === 'page_version_conflict') {
          const mapped = mapApiError(err, 'Error saving page')
          toast.error(mapped.message, {
            duration: 10000,
            testId: 'page-save-version-conflict-toast',
            action: {
              label: (
                <span data-testid="page-save-version-conflict-action">
                  Save anyway
                </span>
              ),
              onClick: () => {
                forceOverwrite()
                  .then((page) => {
                    if (page) {
                      window.history.replaceState(
                        null,
                        '',
                        buildBrowserEditUrl(`/${page.path}`),
                      )
                      toast.success('Page saved successfully')
                    }
                  })
                  .catch((overwriteErr) => {
                    const overwriteLocalized = asApiLocalizedError(overwriteErr)
                    if (overwriteLocalized?.code === 'page_version_conflict') {
                      toast.error(
                        'The page was modified again while saving. Please reload the page and re-apply your changes.',
                        { duration: 8000 },
                      )
                    } else {
                      const overwriteMapped = mapApiError(
                        overwriteErr,
                        'Error saving page',
                      )
                      toast.error(overwriteMapped.message)
                    }
                  })
              },
            },
          })
        } else {
          const mapped = mapApiError(err, 'Error saving page')
          toast.error(mapped.message)
        }
      })
  }, [savePage, forceOverwrite])

  const handleClose = useCallback(() => {
    const state = usePageEditorStore.getState()
    const currentPage = state.page
    const hasUnsavedChanges = isDirtyState(state)

    if (!hasUnsavedChanges) {
      // Saving updates the editor store before React finishes re-rendering.
      // Skip the blocker for this close action when the latest store snapshot
      // is already clean.
      skipNavigationGuardRef.current = true
    }

    if (currentPage?.path) {
      navigate(`/${currentPage.path}`, {
        state: createNavigationVisitState(),
      })
    } else {
      navigate('/', { state: createNavigationVisitState() })
    }
  }, [navigate])

  const openLinkDialog = useCallback(() => {
    const view = editorRef.current?.editorViewRef.current
    const selectedText = view
      ? view.state.doc.sliceString(
          view.state.selection.main.from,
          view.state.selection.main.to,
        )
      : ''
    openDialog(DIALOG_LINK_INSERT, { editorRef, selectedText })
  }, [editorRef, openDialog])

  // register toolbar actions
  useToolbarActions({
    savePage: () => handleSave(),
    closePage: handleClose,
    formatBold: () => editorRef.current?.insertWrappedText('**', '**'),
    formatItalic: () => editorRef.current?.insertWrappedText('_', '_'),
    formatInlineCode: () => editorRef.current?.insertWrappedText('`', '`'),
    openLinkDialog,
    insertHeading: (level) => editorRef.current?.insertHeading(level),
    getEditorView: () => editorRef.current?.editorViewRef.current ?? null,
  })

  // content changes in the editor are synced to the store
  const handleEditorChange = useCallback(
    (value: string) => {
      setContent(value) // store update
    },
    [setContent],
  )

  if (notFound) {
    return <Page404 targetPath={getWikiTargetRoutePath(pathname)} />
  }

  if (error) return <p className="page-editor__error">Error: {error}</p>

  return (
    <>
      <div className="page-editor">
        {initialPage && (
          <>
            <PageFrontmatterPanel
              tags={tags}
              fields={frontmatterFields}
              errors={frontmatterErrors}
              hasUnsupportedFields={Boolean(frontmatterUnsupported)}
              onTagsChange={setTags}
              onFieldsChange={setFrontmatterFields}
            />
            <MarkdownEditor
              ref={editorRef}
              pageId={initialPage.id}
              initialValue={initialPage.content || ''}
              onChange={handleEditorChange}
            />
          </>
        )}
      </div>
    </>
  )
}
