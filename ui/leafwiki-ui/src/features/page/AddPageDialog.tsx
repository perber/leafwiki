import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { createPage, suggestSlug } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDebounce } from '@/lib/useDebounce'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

type AddPageDialogProps = {
  parentId: string
}

export function AddPageDialog({ parentId }: AddPageDialogProps) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === 'add')

  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [loading, setLoading] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')
  const navigate = useNavigate()

  const debouncedTitle = useDebounce(title, 300)

  useEffect(() => {
    if (debouncedTitle.trim() === '') return
    const generateSlug = async () => {
      try {
        const suggestion = await suggestSlug(parentId, debouncedTitle)
        setSlug(suggestion)
      } catch (err) {
        toast.error('Error generating slug')
      }
    }

    generateSlug()
  }, [debouncedTitle, parentId])

  const handleTitleChange = async (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
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
      closeDialog()
      resetForm()
    } catch (err: any) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      setLoading(false)
    }
  }

  const handleCancel = () => {
    resetForm()
    closeDialog()
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
        if (!isOpen) {
          resetForm()
          closeDialog()
        }
      }}
    >
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
