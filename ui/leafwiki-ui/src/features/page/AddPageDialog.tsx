import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { createPage, suggestSlug } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useTreeStore } from '@/stores/tree'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
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
  const navigate = useNavigate()

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
      toast.success('Page created')
      await reloadTree()
      const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
      navigate(`/e/${fullPath}`)
      setOpen(false)
      resetForm()
    } catch (err: any) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      setLoading(false)
    }
  }

  const handleCancel = () => {
    resetForm()
    setOpen(false)
  }

  const resetForm = () => {
    setTitle('')
    setSlug('')
    setFieldErrors({})
    setLoading(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        setOpen(isOpen)
        if (!isOpen) resetForm()
      }}
    >
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
      <DialogContent
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey && !loading && title && slug) {
            e.preventDefault()
            handleCreate()
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>Create a new page</DialogTitle>
          <DialogDescription>Enter the title of the new page</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <FormInput
            autoFocus={true}
            label="Title"
            value={title}
            onChange={(val) => {
              handleTitleChange(val)
              setFieldErrors((prev) => ({ ...prev, title: '' }))
            }}
            placeholder="Page title"
            error={fieldErrors.title}
          />

          <FormInput
            label="Slug"
            value={slug}
            onChange={(val) => {
              setSlug(val)
              setFieldErrors((prev) => ({ ...prev, slug: '' }))
            }}
            placeholder="Page slug"
            error={fieldErrors.slug}
          />
        </div>
        <span className="text-sm text-gray-500">
          Path: {parentPath !== '' && `${parentPath}/`}
          {slug && `${slug}`}
        </span>
        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={handleCancel}
            onSave={handleCreate}
            saveLabel={loading ? 'Creating…' : 'Create'}
            disabled={!title || !slug || loading}
            loading={loading}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
