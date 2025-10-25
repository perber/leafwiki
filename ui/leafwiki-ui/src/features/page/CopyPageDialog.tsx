import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { createPage, Page, PageNode, updatePage } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { JSX, useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

export function CopyPageDialog({
  sourcePage,
}: {
  sourcePage: Page
}) {
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
    console.log('[CopyPageDialog] Setting target parent ID to source page parent ID:', parentId)
    if (parentId) {
      setTargetParentID(parentId)
    }
  }, [parentId])

  const handleCancel = () => {
    resetForm()
    closeDialog()
  }

  const handleCopy = async () => {
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
      const newPage = await createPage({ title, slug, parentId: targetParentID  })

      const p = newPage as Page
      await updatePage(
        p.id,
        title,
        slug,
        sourcePage.content,
      )
      toast.success('Page copied')
      await reloadTree()
      closeDialog()
      resetForm()
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating page')
      setLoading(false)
    }
  }

  const selectOptions = useMemo(() => {
    if (!tree) return null
    const renderOptions = (node: PageNode, depth = 1): JSX.Element[] => {
      const indent = '—'.repeat(depth)
      const options = [
        <SelectItem key={node.id} value={node.id}>
          {indent} {node.title}
        </SelectItem>,
      ]
      if (node.children?.length) {
        for (const child of node.children) {
          options.push(...renderOptions(child, depth + 1))
        }
      }
      return options
    }
    return (
      <>
        <SelectItem key="root" value="root">
          ⬆️ Top Level
        </SelectItem>
        {tree.children?.flatMap((child) => renderOptions(child))}
      </>
    )
  }, [tree])

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
        <Select value={targetParentID} onValueChange={setTargetParentID}>
          <SelectTrigger>
            <SelectValue placeholder="Select new parent..." />
          </SelectTrigger>
          <SelectContent>
            {selectOptions}
          </SelectContent>
        </Select>
        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={handleCancel}
            onSave={async () => await handleCopy()}
            saveLabel={loading ? 'Copying...' : 'Copy Page'}
            disabled={isCopyButtonDisabled}
            loading={loading}
          >

          </FormActions>
        </div>
      </DialogContent>
    </Dialog>
  )
}
