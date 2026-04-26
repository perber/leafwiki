import Page404 from '@/components/Page404'
import { mapApiError, asApiLocalizedError } from '@/lib/api/errors'
import { buildBrowserEditUrl } from '@/lib/routePath'
import { getWikiTargetRoutePath } from '@/lib/wikiPath'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useRef } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useProgressbarStore } from '../progressbar/progressbar'
import MarkdownEditor, { MarkdownEditorRef } from './MarkdownEditor'
import { usePageEditorStore } from './pageEditor'
import useNavigationGuard from './useNavigationGuard'
import { useToolbarActions } from './useToolbarActions'

export default function PageEditor() {
  const { '*': path } = useParams()

  const { pathname } = useLocation()
  const navigate = useNavigate()
  const editorRef = useRef<MarkdownEditorRef>(null)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const savePage = usePageEditorStore((s) => s.savePage)
  const forceOverwrite = usePageEditorStore((s) => s.forceOverwrite)
  const setContent = usePageEditorStore((s) => s.setContent)
  const loadPageData = usePageEditorStore((s) => s.loadPageData)
  const initialPage = usePageEditorStore((s) => s.initialPage) // contains the initial page data when loaded
  const notFound = usePageEditorStore((s) => s.notFound)
  const loading = useProgressbarStore((s) => s.loading)
  const error = usePageEditorStore((s) => s.error)
  const page = usePageEditorStore((s) => s.page)
  const openNode = useTreeStore((s) => s.openNode)
  const dirty = usePageEditorStore((s) => {
    const { page, title, slug, content } = s
    if (!page) return false
    return (
      page.title !== title || page.slug !== slug || page.content !== content
    )
  })

  // Shows Unsaved Changes Dialog when navigating away with dirty state
  useNavigationGuard({
    when: dirty,
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
    if (page?.path) {
      navigate(`/${page.path}`)
    } else {
      navigate('/')
    }
  }, [page, navigate])

  // register toolbar actions
  useToolbarActions({
    savePage: () => handleSave(),
    closePage: handleClose,
    formatBold: () => editorRef.current?.insertWrappedText('**', '**'),
    formatItalic: () => editorRef.current?.insertWrappedText('_', '_'),
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

  if (!initialPage && !loading)
    return <Page404 targetPath={getWikiTargetRoutePath(pathname)} />

  return (
    <>
      <div className="page-editor">
        {initialPage && (
          <MarkdownEditor
            ref={editorRef}
            pageId={initialPage.id}
            initialValue={initialPage.content || ''}
            onChange={handleEditorChange}
          />
        )}
      </div>
    </>
  )
}
