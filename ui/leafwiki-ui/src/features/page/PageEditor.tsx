import { EditorTitleBar } from '@/components/EditorTitleBar'
import MarkdownEditor from '@/components/MarkdownEditor'
import { usePageToolbar } from '@/components/PageToolbarContext'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { UnsavedChangesDialog } from '@/components/UnsavedChangesDialog'
import { getPageByPath, suggestSlug, updatePage } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDebounce } from '@/lib/useDebounce'
import { useTreeStore } from '@/stores/tree'
import { DialogDescription } from '@radix-ui/react-dialog'
import { Save, X } from 'lucide-react'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import { AssetManager } from './AssetManager'

export default function PageEditor() {
  const { '*': path } = useParams()
  const [page, setPage] = useState<any>(null)
  const [inserted, setInserted] = useState<string | null>(null)
  const navigate = useNavigate()
  const [markdown, setMarkdown] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [assetModalOpen, setAssetModalOpen] = useState(false)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const [_, setFieldErrors] = useState<Record<string, string>>({})
  const [showUnsavedDialog, setShowUnsavedDialog] = useState(false)
  const [pendingNavigation, setPendingNavigation] = useState<string | null>(null)


  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')

  const { setContent, clearContent, setTitleBar, clearTitleBar } = usePageToolbar()

  const handleSaveRef = useRef<() => void>(() => { })

  const parentPath =
    useTreeStore(() => {
      const parts = page?.path?.split('/')
      parts?.pop()
      return parts?.join('/')
    }) || ''


  const isDirty = useMemo(() => {
    if (!page) return false
    return (
      title !== page.title ||
      slug !== page.slug ||
      markdown !== page.content
    )
  }, [page, title, slug, markdown])


  const handleNavigateAway = useCallback((targetPath: string) => {
    if (isDirty) {
      setPendingNavigation(targetPath)
      setShowUnsavedDialog(true)
    } else {
      navigate(targetPath)
    }
  }, [isDirty, navigate])

  /**
   * FIXME - Debounce is happending two times, also the generation of slugs is happening when the user is coming in
   * This shouldn't happend and needs to be fixed!
   */
  const debouncedTitle = useDebounce(title, 300)
  useEffect(() => {
    if (!debouncedTitle.trim()) return

    suggestSlug(page?.parentId || '', debouncedTitle)
      .then((suggested) => {
        if (suggested !== slug) {
          setSlug(suggested)
        }
      })
      .catch((err) => console.warn('Slug error', err))
  }, [debouncedTitle, page?.parentId])

  useEffect(() => {
    handleSaveRef.current = async () => {
      if (!isDirty || !page) return

      try {
        toast.info('Saving page...')
        await updatePage(page.id, title, slug, markdown)
        toast.success('Page saved successfully!')
        // Set new page content after save
        setPage({
          ...page,
          title,
          slug,
          content: markdown,
        })
      } catch (err) {
        handleFieldErrors(err, setFieldErrors, 'Error saving page')
      }
    }
  }, [page, title, slug, markdown, parentPath, isDirty])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        handleSaveRef.current()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => {
    if (!page || !title || !slug) return
    setTitleBar(
      <EditorTitleBar
        title={title}
        slug={slug}
        onChange={(newTitle) => {
          setTitle(newTitle)
        }}
      />
    )

    return () => {
      clearTitleBar()
    }
  }, [title, slug, page?.parentId])

  useEffect(() => {
    if (!page) return
    setContent(
      <React.Fragment key="editing">
        <Button
          variant="destructive"
          className="h-8 w-8 rounded-full shadow-sm"
          size="icon"
          onClick={async () => {
            await reloadTree()
            handleNavigateAway(`/${path}`)
          }}
        >
          <X />
        </Button>
        <Button
          onClick={() => handleSaveRef.current()}
          variant="default"
          className="h-8 w-8 rounded-full shadow-md"
          size="icon"
          disabled={!isDirty}
        >
          <Save />
        </Button>
      </React.Fragment>
    )

    return () => {
      clearContent()
    }
  }, [page, setContent, isDirty])

  useEffect(() => {
    if (!path) return

    setLoading(true)
    getPageByPath(path)
      .then((resp) => {
        setPage(resp)
        setMarkdown(resp.content)
        setTitle(resp.title)
        setSlug(resp.slug)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [path])


  useEffect(() => {
    if (title) {
      document.title = `${title} - Edit Page – LeafWiki`
    } else {
      document.title = 'Edit Page – LeafWiki'
    }

    return () => {
      // optional zurücksetzen oder leer lassen
      document.title = 'LeafWiki'
    }
  }, [title])


  if (loading) return <><p className="text-sm text-gray-500">Loading...</p></>
  if (error) return <><p className="text-sm text-red-500">Error: {error}</p></>
  if (!page) return <><p className="text-sm text-gray-500">No page found</p></>

  return (
    <>
      <div className="flex h-[calc(100vh-140px)] gap-6">
        <div className="flex flex-1 flex-col gap-2">
          <div className="flex justify-end pb-2">
            <Button variant="outline" onClick={() => setAssetModalOpen(true)}>
              + Add Asset
            </Button>
          </div>
          <MarkdownEditor
            value={markdown}
            onChange={(val) => {
              setMarkdown(val)

              if (inserted) setInserted(null) // reset after insert
            }}
            insert={inserted}
          />
        </div>
        <Dialog open={assetModalOpen} onOpenChange={setAssetModalOpen}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>Add Asset</DialogTitle>
              <DialogDescription>
                Upload or select an asset to insert into the page.
              </DialogDescription>
            </DialogHeader>
            <AssetManager
              pageId={page.id}
              onInsert={(md) => {
                setInserted(md)
                setAssetModalOpen(false)
              }}
            />
          </DialogContent>
        </Dialog>
      </div>
      <UnsavedChangesDialog
        open={showUnsavedDialog}
        onCancel={() => {
          setShowUnsavedDialog(false)
          setPendingNavigation(null)
        }}
        onConfirm={() => {
          if (pendingNavigation) {
            setShowUnsavedDialog(false)
            navigate(pendingNavigation)
          }
        }}
      />
    </>
  )
}
