import Page404 from '@/components/Page404'
import Loader from '@/components/PageLoader'
import { buildEditUrl } from '@/lib/urlUtil'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import MarkdownEditor, { MarkdownEditorRef } from './MarkdownEditor'
import { usePageEditorStore } from './pageEditor'
import useNavigationGuard from './useNavigationGuard'
import { useToolbarActions } from './useToolbarActions'

export default function PageEditor() {
  const { '*': path } = useParams()

  const navigate = useNavigate()
  const editorRef = useRef<MarkdownEditorRef>(null)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const savePage = usePageEditorStore((s) => s.savePage)
  const setContent = usePageEditorStore((s) => s.setContent)
  const loadPageData = usePageEditorStore((s) => s.loadPageData)
  const initialPage = usePageEditorStore((s) => s.initialPage) // contains the initial page data when loaded
  const loading = usePageEditorStore((s) => s.loading)
  const error = usePageEditorStore((s) => s.error)
  const page = usePageEditorStore((s) => s.page)
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

  // callbacks to save / close
  const handleSave = useCallback(() => {
    savePage()
      .then(async (page) => {
        // update URL the new path after save without reloading
        if (page) {
          window.history.replaceState(null, '', buildEditUrl(`/${page?.path}`))
          toast.success('Page saved successfully')
        }
      })
      .catch(() => {
        toast.error('Error saving page')
      })
  }, [savePage])
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
  })

  // content changes in the editor are synced to the store
  const handleEditorChange = useCallback(
    (value: string) => {
      setContent(value) // store update
    },
    [setContent],
  )

  if (loading)
    return (
      <div className="p-6">
        <Loader />
      </div>
    )

  if (error) return <p className="p-6 text-sm text-red-500">Error: {error}</p>

  if (!initialPage)
    return (
      <div className="p-6">
        <Page404 />
      </div>
    )

  return (
    <>
      <div className="pageEditor h-full w-full overflow-hidden">
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
