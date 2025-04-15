import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { suggestSlug } from '@/lib/api'
import { useDebounce } from '@/lib/useDebounce'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

type EditPageMetadataDialogProps = {
  parentId: string
  title: string
  slug: string
  onChange: (title: string, slug: string) => void
}

export function EditPageMetadataDialog({ parentId, title: propTitle, slug: propSlug, onChange }: EditPageMetadataDialogProps) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')
  const open = useDialogsStore((s) => s.dialogType === 'edit-page-metadata')

  const [title, setTitle] = useState(propTitle)
  const [slug, setSlug] = useState(propSlug)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const debouncedTitle = useDebounce(title, 300)

  useEffect(() => {
    // If the title is the same as the propTitle, do not suggest a new slug
    // This prevents the slug from being suggested when the title is the same
    // This occurs when the user opens the dialog and the title is already set
    if (propTitle === title) {
      return
    }

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
  }, [debouncedTitle, parentId, propTitle])

  const handleTitleChange = async (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }

  const handleCancel = () => {
    resetForm()
    closeDialog()
  }

  const resetForm = () => {
    setTitle(propTitle)
    setSlug(propSlug)
    setFieldErrors({})
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
          if (e.key === 'Enter' && !e.shiftKey && title && slug) {
            e.preventDefault()
            onChange(title, slug)
            closeDialog()
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>Edit page metadata</DialogTitle>
          <DialogDescription>Change metadata of the page</DialogDescription>
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
            onSave={() => {
              onChange(title, slug)
              closeDialog()
            }}
            saveLabel={'Change'}
            disabled={!title || !slug}
            loading={false}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}