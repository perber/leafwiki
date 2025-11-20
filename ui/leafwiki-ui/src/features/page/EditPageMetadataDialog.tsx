import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { DIALOG_EDIT_PAGE_METADATA } from '@/lib/registries'
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
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')

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

  const resetForm = () => {
    setTitle(propTitle)
    setSlug(propSlug)
    setSlugTouched(false)
    setLastSlugTitle('')
    setFieldErrors({})
  }

  return (
    <BaseDialog
      dialogType={DIALOG_EDIT_PAGE_METADATA}
      dialogTitle="Edit page metadata"
      dialogDescription="Change metadata of the page"
      onClose={() => {
        resetForm()
        return true
      }}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          onChange(title, slug)
          return true
        }
        return false
      }}
      cancelButton={{ label: 'Cancel', variant: 'outline', autoFocus: false }}
      buttons={[{ label: 'Change', actionType: 'confirm', disabled: isSaveDisabled, variant: 'default', autoFocus: true }]}
    >
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
    </BaseDialog>
  )
}
