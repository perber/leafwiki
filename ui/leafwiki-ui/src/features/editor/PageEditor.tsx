import Page404 from '@/components/Page404'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { mapApiError, asApiLocalizedError } from '@/lib/api/errors'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { buildBrowserEditUrl, buildViewUrl } from '@/lib/routePath'
import { DIALOG_LINK_INSERT } from '@/lib/registries'
import { getWikiTargetRoutePath } from '@/lib/wikiPath'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'
import MarkdownEditor, { MarkdownEditorRef } from './MarkdownEditor'
import { PageFrontmatterPanel } from './PageFrontmatterPanel'
import { usePageEditorStore } from './pageEditorStore'
import { isDirtyState } from './pageEditorStore'
import { useAutoSave } from './useAutoSave'
import useNavigationGuard from './useNavigationGuard'
import { useToolbarActions } from './useToolbarActions'

export default function PageEditor() {
  const { t } = useTranslation('editor')
  const { '*': path, draftId } = useParams()

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
  const loadPendingDraft = usePageEditorStore((s) => s.loadPendingDraft)
  const isDraft = usePageEditorStore((s) => s.isDraft)
  const publishDraft = usePageEditorStore((s) => s.publishDraft)
  const discardDraft = usePageEditorStore((s) => s.discardDraft)
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
  const { status: autoSaveStatus } = useAutoSave()

  // Shows Unsaved Changes Dialog when navigating away with dirty state
  useNavigationGuard({
    when: () => dirty && !skipNavigationGuardRef.current,
    onNavigate: async () => {
      await reloadTree()
    },
  })

  // Load page data when path changes
  useEffect(() => {
    if (draftId) loadPendingDraft(draftId)
    else if (path) loadPageData(path)
  }, [path, draftId, loadPageData, loadPendingDraft])

  // Open node
  useEffect(() => {
    if (!initialPage?.id) return
    openNode(initialPage.id)
  }, [openNode, initialPage?.id])

  // Reset the editor store on unmount so stale `page` data (and thus
  // currentEditorPageId reads elsewhere) doesn't outlive the editor session.
  // Declared after useAutoSave() so its cleanup runs after useAutoSave's own
  // unmount cleanup, which may synchronously kick off a flush save that reads
  // store state before it's cleared here.
  useEffect(() => {
    return () => {
      usePageEditorStore.getState().resetEditorState()
    }
  }, [])

  // callbacks to save / close
  const handleSave = useCallback(() => {
    savePage()
      .then(async (page) => {
        if (page) {
          if (!usePageEditorStore.getState().isPendingDraft) {
            window.history.replaceState(
              null,
              '',
              buildBrowserEditUrl(`/${page.path}`),
            )
          }
          toast.success(t('pageEditor.savedToast'))
        }
      })
      .catch((err) => {
        const localized = asApiLocalizedError(err)
        if (localized?.code === 'page_version_conflict') {
          const mapped = mapApiError(err, t('pageEditor.saveErrorFallback'))
          toast.error(mapped.message, {
            duration: 10000,
            testId: 'page-save-version-conflict-toast',
            action: {
              label: (
                <span data-testid="page-save-version-conflict-action">
                  {t('pageEditor.saveAnyway')}
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
                      toast.success(t('pageEditor.savedToast'))
                    }
                  })
                  .catch((overwriteErr) => {
                    const overwriteLocalized = asApiLocalizedError(overwriteErr)
                    if (overwriteLocalized?.code === 'page_version_conflict') {
                      toast.error(t('pageEditor.conflictAgainMessage'), {
                        duration: 8000,
                      })
                    } else {
                      const overwriteMapped = mapApiError(
                        overwriteErr,
                        t('pageEditor.saveErrorFallback'),
                      )
                      toast.error(overwriteMapped.message)
                    }
                  })
              },
            },
          })
        } else {
          const mapped = mapApiError(err, t('pageEditor.saveErrorFallback'))
          toast.error(mapped.message)
        }
      })
  }, [savePage, forceOverwrite, t])

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

    if (state.isPendingDraft) {
      const parent = state.pendingParentId
        ? useTreeStore.getState().getPathById(state.pendingParentId)
        : ''
      navigate(parent ? `/${parent}` : '/', {
        state: createNavigationVisitState(),
      })
    } else if (currentPage?.path) {
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

  const handlePublishDraft = useCallback(() => {
    publishDraft()
      .then(async (page) => {
        if (page) {
          await reloadTree()
          navigate(buildViewUrl(page.path), { replace: true })
          toast.success(t('draft.published'))
        }
      })
      .catch((err) =>
        toast.error(
          mapApiError(err, t('pageEditor.saveErrorFallback')).message,
        ),
      )
  }, [publishDraft, reloadTree, t, navigate])

  const handleDiscardDraft = useCallback(() => {
    const state = usePageEditorStore.getState()
    const discardedPagePath = state.page?.path ?? '/'
    const discardedPendingDraft = state.isPendingDraft
    const discardedPendingParentId = state.pendingParentId
    discardDraft()
      .then(async () => {
        await reloadTree()
        if (discardedPendingDraft) {
          const parent = discardedPendingParentId
            ? useTreeStore.getState().getPathById(discardedPendingParentId)
            : ''
          navigate(parent ? `/${parent}` : '/', { replace: true })
        } else {
          navigate(buildViewUrl(discardedPagePath), { replace: true })
        }
        toast.success(t('draft.discarded'))
      })
      .catch((err) =>
        toast.error(
          mapApiError(err, t('pageEditor.saveErrorFallback')).message,
        ),
      )
  }, [discardDraft, reloadTree, t, navigate])

  if (notFound) {
    return <Page404 targetPath={getWikiTargetRoutePath(pathname)} />
  }

  if (error)
    return (
      <p className="page-editor__error">
        {t('pageEditor.errorPrefix', { error })}
      </p>
    )

  return (
    <>
      <div className="page-editor">
        {initialPage && (
          <>
            {isDraft && (
              <div className="page-editor__draft-actions">
                <>
                  <span className="text-sm font-medium">
                    {t('draft.editing')}
                  </span>
                  <Button
                    type="button"
                    size="sm"
                    onClick={handlePublishDraft}
                    disabled={autoSaveStatus === 'saving'}
                  >
                    {t('draft.publish')}
                  </Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={autoSaveStatus === 'saving'}
                      >
                        {t('draft.discard')}
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>
                          {t('draft.discardTitle')}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                          {t('draft.discardDescription')}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>
                          {t('draft.cancel')}
                        </AlertDialogCancel>
                        <AlertDialogAction
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                          onClick={handleDiscardDraft}
                        >
                          {t('draft.discard')}
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </>
              </div>
            )}
            <PageFrontmatterPanel
              tags={tags}
              fields={frontmatterFields}
              errors={frontmatterErrors}
              hasUnsupportedFields={Boolean(frontmatterUnsupported)}
              onTagsChange={setTags}
              onFieldsChange={setFrontmatterFields}
            />
            <MarkdownEditor
              key={`${initialPage.id}:${isDraft ? 'draft' : 'published'}`}
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
