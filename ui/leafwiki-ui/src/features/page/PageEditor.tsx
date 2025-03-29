import MarkdownEditor from '@/components/MarkdownEditor'
import { Button } from '@/components/ui/button'
import { Input } from "@/components/ui/input"
import { getPageByPath, suggestSlug, updatePage } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

export default function PageEditor() {
    const { '*': path } = useParams()
    const [page, setPage] = useState<any>(null)
    const navigate = useNavigate()
    const [markdown, setMarkdown] = useState('')
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const reloadTree = useTreeStore((s) => s.reloadTree)

    const [title, setTitle] = useState('')
    const [slug, setSlug] = useState('')

    const parentPath = useTreeStore(() => {
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

    const handleOnChange = (value: string) => {
        setMarkdown(value)
    }

    const handleSave = async () => {
        try {
            await updatePage(page.id, title, slug, markdown)
            navigate(`/${parentPath}/${slug}`)
            await reloadTree()
        } catch (err) {
            console.error("Save failed", err)
        }
    }

    if (loading) return <p className="text-sm text-gray-500">Loading...</p>
    if (error) return <p className="text-sm text-red-500">Error: {error}</p>
    if (!page) return <p className="text-sm text-gray-500">No page found</p>

    return (
        <div className="flex h-full flex-col gap-4">
            <div className="space-y-2">
                <Input
                    placeholder="Title"
                    className="text-xl font-semibold"
                    value={title}
                    onChange={async (e) => {
                        const val = e.target.value
                        setTitle(val)

                        if (val.trim()) {
                            try {
                                const suggested = await suggestSlug(page?.parentId || '', val)
                                setSlug(suggested)
                            } catch (err) {
                                console.warn("Slug error", err)
                            }
                        } else {
                            setSlug('')
                        }
                    }}
                />
                <p className="text-sm text-gray-500">
                    Path: {parentPath && `${parentPath}/`}{slug}
                </p>
            </div>
            <MarkdownEditor value={markdown} onChange={handleOnChange} />
            <div className="mt-4 flex justify-end">
                <Button variant="outline" onClick={() => navigate(`/${path}`)} className='mr-2'>Cancel</Button>
                <Button onClick={handleSave}>Save</Button>
            </div>
        </div>
    )
}
