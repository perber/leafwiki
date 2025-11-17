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
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/urlUtil'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
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
  const open = useDialogsStore((s) => s.dialogType === DIALOG_ADD_PAGE)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
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

  const resetForm = useCallback(() => {
    setTitle('')
    setSlug('')
    setSlugTouched(false)
    setLastSlugTitle('')
    setFieldErrors({})
    setLoading(false)
  }, [])

  const handleSlugChange = useCallback((val: string) => {
    setSlug(val)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const handleCreate = useCallback(async (redirect: boolean = true) => {
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
        navigate(buildEditUrl(fullPath))
      }
      closeDialog()
      resetForm()
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      setLoading(false)
    }
  }, [title, slug, parentId, slugTouched, slugLoading, lastSlugTitle, reloadTree, parentPath, navigate, closeDialog, resetForm])

  const handleCancel = useCallback(() => {
    resetForm()
    closeDialog()
  }, [resetForm, closeDialog])

  useEffect(() => {
    const closeHotKey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: open,
      action: () => {
        handleCancel()
      },
    }

    const submitHotKey: HotKeyDefinition = {
      keyCombo: 'Enter',
      enabled: open && !isCreateButtonDisabled,
      action: async () => {
        await handleCreate()
      },
    }

    registerHotkey(closeHotKey)
    registerHotkey(submitHotKey)

    return () => {
      unregisterHotkey(closeHotKey.keyCombo)
      unregisterHotkey(submitHotKey.keyCombo)
    }
  }, [open, isCreateButtonDisabled, handleCreate, handleCancel, registerHotkey, unregisterHotkey])

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
      <DialogContent>
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
            testid="add-page-title-input"
            placeholder="Page title"
            error={fieldErrors.title}
          />
          <SlugInputWithSuggestion
            title={title}
            slug={slug}
            testid="add-page-slug-input"
            parentId={parentId}
            onSlugChange={handleSlugChange}
            onSlugTouchedChange={setSlugTouched}
            onSlugLoadingChange={setSlugLoading}
            onLastSlugTitleChange={setLastSlugTitle}
            error={fieldErrors.slug}
          />
        </div>
        <span
          className="text-sm text-gray-500"
          data-testid="add-page-path-display"
        >
          Path: {parentPath !== '' && `${parentPath}/`}
          {slug && `${slug}`}
        </span>
        <div className="mt-4 flex justify-end">
          <FormActions
            testidPrefix="add-page-dialog"
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
              data-testid="add-page-create-button-without-redirect"
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
