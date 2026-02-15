import Page404 from '@/components/Page404'
import { buildEditUrl } from '@/lib/urlUtil'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { useProgressbarStore } from '../progressbar/progressbar'
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
    if (!page) return
    openNode(page.id)
  }, [openNode, page])

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

  if (error) return <p className="page-editor__error">Error: {error}</p>

  if (!initialPage && !loading)
    return (
      <div className="page-editor__not-found">
        <Page404 />
      </div>
    )

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
