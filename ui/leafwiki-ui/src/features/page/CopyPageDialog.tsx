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
import { copyPage, Page, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { Loader2 } from 'lucide-react'
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
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === 'copy-page')
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
    console.log(
      '[CopyPageDialog] Setting target parent ID to source page parent ID:',
      parentId,
    )
    if (parentId) {
      setTargetParentID(parentId)
    }
  }, [parentId])

  const handleCancel = () => {
    resetForm()
    closeDialog()
  }

  const handleCopy = async (redirect: boolean) => {
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
      await copyPage(sourcePage.id, targetParentID, title, slug)
      toast.success('Page copied')
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

  useEffect(() => {
    if (sourcePage && sourcePage.title) {
      setTitle(`Copy of ${sourcePage.title}`)
    }
  }, [sourcePage])

  if (!sourcePage) return null

  if (!tree) return null

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => {
        if (!open) {
          closeDialog()
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Copy Page</DialogTitle>
        </DialogHeader>
        <DialogDescription>Create a copy of this page</DialogDescription>
        <FormInput
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
        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={handleCancel}
            onSave={async () => await handleCopy(true)}
            saveLabel={loading ? 'Copying...' : 'Copy & Edit Page'}
            disabled={isCopyButtonDisabled}
            loading={loading}
          >
            <Button
              onClick={async () => await handleCopy(false)}
              variant="default"
              disabled={isCopyButtonDisabled}
            >
              {loading ? 'Copying...' : 'Copy'}{' '}
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
