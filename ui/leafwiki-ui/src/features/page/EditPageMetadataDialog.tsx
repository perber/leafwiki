import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useState } from 'react'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

type EditPageMetadataDialogProps = {
  parentId: string
  currentId?: string
  title: string
  slug: string
  onChange: (title: string, slug: string) => void
}

export function EditPageMetadataDialog({
  parentId,
  currentId,
  title: propTitle,
  slug: propSlug,
  onChange,
}: EditPageMetadataDialogProps) {
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')
  const open = useDialogsStore((s) => s.dialogType === 'edit-page-metadata')

  const [title, setTitle] = useState(propTitle)
  const [slug, setSlug] = useState(propSlug)
  const [slugTouched, setSlugTouched] = useState(false)
  const [slugLoading, setSlugLoading] = useState(false)
  const [lastSlugTitle, setLastSlugTitle] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const isSaveDisabled =
    !title ||
    !slug ||
    (!slugTouched && (slugLoading || title !== lastSlugTitle))

  const handleTitleChange = (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }

  const handleSlugChange = useCallback((val: string) => {
    setSlug(val)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const handleCancel = () => {
    resetForm()
    closeDialog()
  }

  const resetForm = () => {
    setTitle(propTitle)
    setSlug(propSlug)
    setSlugTouched(false)
    setLastSlugTitle('')
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
          if (e.key === 'Enter' && !e.shiftKey && !isSaveDisabled) {
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
            autoFocus
            label="Title"
            value={title}
            onChange={handleTitleChange}
            placeholder="Page title"
            error={fieldErrors.title}
          />

          <SlugInputWithSuggestion
            title={title}
            slug={slug}
            currentId={currentId}
            parentId={parentId}
            initialTitle={propTitle}
            enableSlugSuggestion={true}
            onSlugChange={handleSlugChange}
            onSlugTouchedChange={setSlugTouched}
            onSlugLoadingChange={setSlugLoading}
            onLastSlugTitleChange={setLastSlugTitle}
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
            saveLabel="Change"
            disabled={isSaveDisabled}
            loading={false}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
