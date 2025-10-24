import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { createPage } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { Loader2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

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
  const [slugLoading, setSlugLoading] = useState(false)
  const [lastSlugTitle, setLastSlugTitle] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')
  const navigate = useNavigate()

  const isCreateButtonDisabled =
    !title ||
    !slug ||
    loading ||
    (!slugTouched && (slugLoading || title !== lastSlugTitle))

  const handleTitleChange = (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }

  useEffect(() => {
    console.debug('[AddPageDialog] slugLoading changed:', slugLoading)
  }, [slugLoading])

  const handleSlugChange = useCallback((val: string) => {
    setSlug(val)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const handleCreate = async (redirect: boolean = true) => {
    if (!title) return

    if (!slug) {
      toast.error('Slug could not be generated. Please enter it manually.')
      return
    }

    if (!slugTouched && (slugLoading || title !== lastSlugTitle)) {
      toast.warning('Please wait until the slug is fully generated.')
      return
    }

    setLoading(true)
    setFieldErrors({})
    try {
      await createPage({ title, slug, parentId })
      toast.success('Page created')
      await reloadTree()
      if (redirect) {
        const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
        navigate(`/e/${fullPath}`)
      }
      closeDialog()
      resetForm()
    } catch (err: unknown) {
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
    setSlugTouched(false)
    setLastSlugTitle('')
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
          if (e.key === 'Enter' && !e.shiftKey && !isCreateButtonDisabled) {
            e.preventDefault()
            handleCreate(true)
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
          <SlugInputWithSuggestion
            title={title}
            slug={slug}
            parentId={parentId}
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
            onSave={async () => await handleCreate(true)}
            saveLabel={loading ? 'Creating…' : 'Create & Edit Page'}
            disabled={isCreateButtonDisabled}
            loading={loading}
          >
            <Button
              onClick={async () => await handleCreate(false)}
              variant="default"
              disabled={isCreateButtonDisabled}
            >
              {loading ? 'Creating...' : 'Create'}{' '}
              {loading &&
                (loading ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : null)}
            </Button>
          </FormActions>
        </div>
      </DialogContent>
    </Dialog>
  )
}
