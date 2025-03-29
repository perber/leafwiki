import MarkdownEditor from '@/components/MarkdownEditor'
import { Button } from '@/components/ui/button'
import { getPageByPath, updatePage } from '@/lib/api'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

export default function PageEditor() {
  const { '*': path } = useParams()
  const [page, setPage] = useState<any>(null)
  const navigate = useNavigate()
  const [markdown, setMarkdown] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!path) return
    setLoading(true)
    getPageByPath(path)
      .then((resp) => {
        setPage(resp)
        setMarkdown(resp.content)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [path])

  const handleOnChange = (value: string) => {
    setMarkdown(value)
  }

  const handleSave = async () => {
    try {
      await updatePage(page.id, page.title, page.slug, markdown)
      navigate(`/${path}`)
    } catch (err) {
      console.error("Save failed", err)
    }
  }

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error) return <p className="text-sm text-red-500">Error: {error}</p>
  if (!page) return <p className="text-sm text-gray-500">No page found</p>

  return (
    <div className="flex h-full flex-col gap-4">
      <h1 className="text-xl font-semibold">{page.title}</h1>
      <MarkdownEditor value={markdown} onChange={handleOnChange}/>
      <div className="mt-4 flex justify-end">
        <Button onClick={handleSave}>Save</Button>
      </div>
    </div>
  )
}
