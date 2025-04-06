import { FormActions } from '@/components/FormActions'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { createPage, suggestSlug } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

type AddPageDialogProps = {
  parentId: string
  minimal?: boolean
}

export function AddPageDialog({ parentId, minimal }: AddPageDialogProps) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [loading, setLoading] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')

  const handleTitleChange = async (val: string) => {
    setTitle(val)
    if (!val.trim()) {
      setSlug('')
      return
    }
    try {
      const suggestion = await suggestSlug(parentId, val)
      setSlug(suggestion)
    } catch (err) {
      toast.error('Error generating slug')
    }
  }

  const handleCreate = async () => {
    if (!title || !slug) return
    setLoading(true)
    setFieldErrors({})
    try {
      await createPage({ title, slug, parentId })
    } catch (err: any) {
      console.warn(err)

      if (err?.error === 'validation_error' && Array.isArray(err.fields)) {
        const errorMap: Record<string, string> = {}
        for (const e of err.fields) {
          errorMap[e.field] = e.message
        }
        setFieldErrors(errorMap)
      }

      if (err instanceof Error) {
        toast.error(err.message)
      } else {
        toast.error('Error creating page')
      }

      setLoading(false)
      return
    }
    toast.success('Page created')
    await reloadTree()
    setLoading(false)
    setOpen(false)
    setTitle('')
    setSlug('')
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {minimal ? (
          <button onClick={() => setOpen(true)}>
            <Plus
              size={16}
              className="cursor-pointer text-gray-500 hover:text-gray-800"
            />
          </button>
        ) : (
          <button onClick={() => setOpen(true)}>
            <Plus
              size={16}
              className="cursor-pointer text-gray-500 hover:text-gray-800"
            />
            Create page {parentId}
          </button>
        )}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create a new page</DialogTitle>
          <DialogDescription>Enter the title of the new page</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <Input
            placeholder="Title"
            value={title}
            onChange={(e) => handleTitleChange(e.target.value)}
            className={fieldErrors.title ? 'border-red-500' : ''}
          />
          {fieldErrors.title && (
            <p className="text-sm text-red-500">{fieldErrors.title}</p>
          )}
          <Input
            placeholder="Slug"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            className={fieldErrors.slug ? 'border-red-500' : ''}
          />
          {fieldErrors.slug && (
            <p className="text-sm text-red-500">{fieldErrors.slug}</p>
          )}
        </div>
        <span className="text-sm text-gray-500">
          Path: {parentPath !== '' && `${parentPath}/`}
          {slug && `${slug}`}
        </span>
        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={() => setOpen(false)}
            onSave={handleCreate}
            saveLabel="Create"
            disabled={!title || !slug || loading}
            loading={loading}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
