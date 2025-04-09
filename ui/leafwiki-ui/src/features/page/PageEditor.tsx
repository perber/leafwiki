import Breadcrumbs from '@/components/Breadcrumbs'
import MarkdownEditor from '@/components/MarkdownEditor'
import { usePageToolbar } from '@/components/PageToolbarContext'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getPageByPath, suggestSlug, updatePage } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { Save, X } from 'lucide-react'
import { useEffect, useState } from 'react'
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
  const reloadTree = useTreeStore((s) => s.reloadTree)

  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')

  const { setContent, clear } = usePageToolbar()

  const parentPath =
    useTreeStore(() => {
      const parts = page?.path?.split('/')
      parts?.pop()
      return parts?.join('/')
    }) || ''

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
    if (!page) return
    console.log('page', page)
    setContent(
      <div className="flex items-center gap-2">
        <Breadcrumbs />
        <Button
          variant="destructive"
          className='rounded-full shadow-sm'
          size="icon"
          onClick={() => navigate(`/${path}`)}
        >
          <X />
        </Button>
        <Button onClick={handleSave} variant="default" className='rounded-full shadow-md' size="icon">
          <Save />
        </Button>
      </div>
    )

    return () => {
      clear()
    }
  }, [page, setContent])

  const handleSave = async () => {
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
        <MarkdownEditor
          value={markdown}
          onChange={(val) => {
            setMarkdown(val)

            if (inserted) setInserted(null) // reset after insert
          }}
          insert={inserted}
        />
      </div>
      <div className="w-64 overflow-y-auto border-l pl-4">
        <AssetManager pageId={page.id} onInsert={(md) => setInserted(md)} />
      </div>
    </div>
  )
}
