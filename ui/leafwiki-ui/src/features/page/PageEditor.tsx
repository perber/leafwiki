import MarkdownEditor from '@/components/MarkdownEditor'
import { usePageToolbar } from '@/components/PageToolbarContext'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { getPageByPath, suggestSlug, updatePage } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { DialogDescription } from '@radix-ui/react-dialog'
import { Save, X } from 'lucide-react'
import React, { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
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

  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')

  const { setContent, clear } = usePageToolbar()

  const handleSaveRef = useRef<() => void>(() => { })

  const parentPath =
    useTreeStore(() => {
      const parts = page?.path?.split('/')
      parts?.pop()
      return parts?.join('/')
    }) || ''

  useEffect(() => {
    handleSaveRef.current = async () => {
      try {
        await updatePage(page.id, title, slug, markdown)
        if (parentPath === '') {
          await reloadTree()
          navigate(`/${slug}`)
          return
        }
        navigate(`/${parentPath}/${slug}`)
        await reloadTree()
      } catch (err) {
        console.error('Save failed', err)
      }
    }
  }, [page, title, slug, markdown, parentPath, reloadTree, navigate])

  useEffect(() => {
    if (!page) return
    setContent(
      <React.Fragment key="editing">
        <Button
          variant="destructive"
          className="h-8 w-8 rounded-full shadow-sm"
          size="icon"
          onClick={() => navigate(`/${path}`)}
        >
          <X />
        </Button>
        <Button
          onClick={() => handleSaveRef.current()}
          variant="default"
          className="h-8 w-8 rounded-full shadow-md"
          size="icon"
        >
          <Save />
        </Button>
      </React.Fragment>,
    )

    return () => {
      clear()
    }
  }, [page, setContent])

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


  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error) return <p className="text-sm text-red-500">Error: {error}</p>
  if (!page) return <p className="text-sm text-gray-500">No page found</p>

  return (
    <div className="flex h-[calc(100vh-120px)] gap-6">
      <div className="flex flex-1 flex-col gap-2">
        <div className="space-y-2">
          <Input
            placeholder="Title"
            className="mb-2 text-xl font-semibold"
            value={title}
            onChange={async (e) => {
              const val = e.target.value
              setTitle(val)

              if (val.trim()) {
                try {
                  const suggested = await suggestSlug(page?.parentId || '', val)
                  setSlug(suggested)
                } catch (err) {
                  console.warn('Slug error', err)
                }
              } else {
                setSlug('')
              }
            }}
          />
          <p className="text-sm text-gray-500">
            Path: {parentPath && `${parentPath}/`}
            {slug}
          </p>
        </div>
        <div className="flex justify-end pb-2">
          <Button variant="outline" onClick={() => setAssetModalOpen(true)}>
            + Add Asset
          </Button>
        </div>
        <MarkdownEditor
          value={markdown}
          onChange={(val) => {
            console.log('Markdown changed', val)
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
  )
}
