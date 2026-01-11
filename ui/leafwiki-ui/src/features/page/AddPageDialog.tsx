import BaseDialog, { BaseDialogConfirmButton } from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { createPage, NODE_KIND_PAGE } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/urlUtil'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

type AddPageDialogProps = {
  parentId: string
  nodeKind?: 'page' | 'section'
}

export function AddPageDialog({ parentId, nodeKind = NODE_KIND_PAGE }: AddPageDialogProps) {
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

  const handleCreate = useCallback(
    async (redirect: boolean = true, nodeKind?: 'page' | 'section'): Promise<boolean> => {
      if (!nodeKind) nodeKind = NODE_KIND_PAGE // Default to 'page' if not provided
      if (!title) return false // Should not happen due to button disabling

      if (!slug) {
        toast.error('Slug could not be generated. Please enter it manually.')
        return false // Should not happen due to button disabling
      }

      if (!slugTouched && (slugLoading || title !== lastSlugTitle)) {
        toast.warning('Please wait until the slug is fully generated.')
        return false // Should not happen due to button disabling
      }

      setLoading(true)
      setFieldErrors({})
      try {
        await createPage({ title, slug, parentId, kind: nodeKind })
        toast.success('Page created')
        await reloadTree()
        if (redirect) {
          const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
          navigate(buildEditUrl(fullPath))
        }
        resetForm()
        return true // Close the dialog
      } catch (err: unknown) {
        console.warn(err)
        handleFieldErrors(err, setFieldErrors, 'Error creating page')
        return false // Keep the dialog open
      } finally {
        setLoading(false)
      }
    },
    [
      title,
      slug,
      parentId,
      slugTouched,
      slugLoading,
      lastSlugTitle,
      reloadTree,
      parentPath,
      navigate,
      resetForm,
    ],
  )

  const handleCancel = useCallback(() => {
    resetForm()
    return true
  }, [resetForm])

  const buttons = useMemo(() => {
    const b: BaseDialogConfirmButton[] = [

      {
        label: 'Create',
        actionType: 'no-redirect',
        autoFocus: true,
        loading,
        disabled: isCreateButtonDisabled,
        variant: nodeKind === NODE_KIND_PAGE ? 'secondary' : 'default',
      },
    ]
    if (nodeKind === NODE_KIND_PAGE) {
      b.push(
        {
          label: 'Create & Edit Page',
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: isCreateButtonDisabled,
          variant: 'default',
        })
    }
    return b
  }, [
    isCreateButtonDisabled,
    loading,
    nodeKind,
  ])

  return (
    <BaseDialog
      dialogTitle={nodeKind === 'page' ? "Create a new page" : "Create a new section"}
      dialogDescription={nodeKind === 'page' ? "Enter the title of the new page" : "Enter the title of the new section"}
      dialogType={DIALOG_ADD_PAGE}
      onClose={handleCancel}
      onConfirm={async (actionType: string): Promise<boolean> => {
        return await handleCreate(actionType !== 'no-redirect')
      }}
      testidPrefix="add-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={buttons}
    >
      <div className="page-dialog__fields">
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
      <span className="dialog__path" data-testid="add-page-path-display">
        Path: {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
