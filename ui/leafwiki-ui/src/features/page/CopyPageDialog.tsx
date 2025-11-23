import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { copyPage, Page, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_COPY_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/urlUtil'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

export function CopyPageDialog({ sourcePage }: { sourcePage: Page }) {
  // Dialog state from zustand store
  const [targetParentID, setTargetParentID] = useState<string>('root')
  const [title, setTitle] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(false)
  const [slug, setSlug] = useState<string>('')
  const [slugLoading, setSlugLoading] = useState<boolean>(false)
  const [slugTouched, setSlugTouched] = useState<boolean>(false)
  const [lastSlugTitle, setLastSlugTitle] = useState<string>('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const parentPath = useTreeStore((s) => s.getPathById(targetParentID) || '')
  const navigate = useNavigate()

  const { tree, reloadTree } = useTreeStore()

  const handleTitleChange = (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }

  const handleSlugChange = useCallback((val: string) => {
    setSlug(val)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const resetForm = () => {
    setTitle('')
    setSlug('')
    setTargetParentID('root')
    setLoading(false)
    setSlugLoading(false)
    setSlugTouched(false)
    setLastSlugTitle('')
    setFieldErrors({})
  }

  const isCopyButtonDisabled =
    !title ||
    !slug ||
    loading ||
    (!slugTouched && (slugLoading || title !== lastSlugTitle))

  const parentId = useMemo(() => {
    const findParent = (node: PageNode): string | null => {
      for (const child of node.children || []) {
        if (child.id === sourcePage.id) return node.id
        const found = findParent(child)
        if (found) return found
      }
      return null
    }

    if (!tree) return null
    return findParent(tree)
  }, [tree, sourcePage.id])

  useEffect(() => {
    if (parentId) {
      setTargetParentID(parentId)
    }
  }, [parentId])

  const handleCancel = () => {
    resetForm()
    return true
  }

  const handleCopy = async (redirect: boolean): Promise<boolean> => {
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
      await copyPage(sourcePage.id, targetParentID, title, slug)
      toast.success('Page copied')
      await reloadTree()
      if (redirect) {
        const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
        navigate(buildEditUrl(fullPath))
      }
      resetForm()
      return true // Close the dialog
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error copying page')
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (sourcePage && sourcePage.title) {
      setTitle(`Copy of ${sourcePage.title}`)
    }
  }, [sourcePage])

  if (!sourcePage) return null

  if (!tree) return null

  return (
    <BaseDialog
      dialogTitle="Copy Page"
      dialogDescription="Create a copy of this page"
      dialogType={DIALOG_COPY_PAGE}
      onClose={handleCancel}
      onConfirm={async (): Promise<boolean> => {
        return await handleCopy(true)
      }}
      testidPrefix="copy-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading ? 'Copying...' : 'Copy & Edit Page',
          actionType: 'confirm',
          autoFocus: true,
          loading,
          disabled: isCopyButtonDisabled,
          variant: 'default',
        },
      ]}
    >
      <FormInput
        testid="copy-page-dialog-title-input"
        autoFocus={true}
        label="Title"
        value={title}
        onChange={(val) => {
          handleTitleChange(val)
        }}
        placeholder="Page title"
        error={fieldErrors.title}
      />
      <SlugInputWithSuggestion
        testid="copy-page-dialog-slug-input"
        title={title}
        slug={slug}
        parentId={targetParentID}
        onSlugChange={handleSlugChange}
        onSlugTouchedChange={setSlugTouched}
        onSlugLoadingChange={setSlugLoading}
        onLastSlugTitleChange={setLastSlugTitle}
        error={fieldErrors.slug}
      />
      <PageSelect pageID={targetParentID} onChange={setTargetParentID} />
      <span className="text-sm text-gray-500">
        Path: {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
