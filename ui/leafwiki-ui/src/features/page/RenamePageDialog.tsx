import BaseDialog, { BaseDialogConfirmButton } from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import {
  applyPageRefactor,
  getPageByPath,
  NODE_KIND_PAGE,
  PageNode,
  PageRefactorPreview,
  previewPageRefactor,
  updatePage,
} from '@/lib/api/pages'
import { DIALOG_RENAME_PAGE } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useTreeStore } from '@/stores/tree'
import { CalendarDays } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'
import { confirmPageRefactor } from './pageRefactorDialogState'
import { refreshAfterPageRefactor } from './pageMutationRefresh'

type RenamePageDialogProps = {
  page: Pick<
    PageNode,
    'id' | 'title' | 'slug' | 'path' | 'version' | 'parentId' | 'kind'
  >
}

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

function getSyntheticRenamePreview({
  page,
  newSlug,
  parentPath,
}: {
  page: RenamePageDialogProps['page']
  newSlug: string
  parentPath: string
}): PageRefactorPreview {
  const normalizedParentPath =
    parentPath && parentPath !== '/' ? parentPath : ''
  const normalizedNewPath = normalizedParentPath
    ? `${normalizedParentPath}/${newSlug}`
    : `/${newSlug}`

  return {
    kind: 'rename',
    pageId: page.id,
    oldPath: page.path,
    newPath: normalizedNewPath,
    affectedPages: [],
    counts: {
      affectedPages: 0,
      matchedLinks: 0,
    },
    warnings: [],
  }
}

export function RenamePageDialog({ page }: RenamePageDialogProps) {
  const [title, setTitle] = useState(page.title)
  const [slug, setSlug] = useState(page.slug)
  const [loading, setLoading] = useState(false)
  const [slugLoading, setSlugLoading] = useState(false)
  const [slugTouched, setSlugTouched] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const reloadTree = useTreeStore((state) => state.reloadTree)
  const parentPath = useTreeStore((state) =>
    page.parentId ? state.getPathById(page.parentId) ?? '' : '',
  )
  const enableLinkRefactor = useConfigStore((state) => state.enableLinkRefactor)
  const navigate = useNavigate()
  const location = useLocation()
  const itemLabel = page.kind === NODE_KIND_PAGE ? 'Page' : 'Section'
  const itemLabelLower = page.kind === NODE_KIND_PAGE ? 'page' : 'section'

  const titleChanged = title !== page.title
  const slugChanged = slug !== page.slug
  const isRenameDisabled =
    !title ||
    !slug ||
    loading ||
    (!slugTouched && slugLoading) ||
    (!titleChanged && !slugChanged)

  const handleTitleChange = useCallback((value: string) => {
    setTitle(value)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }, [])

  const handleSlugChange = useCallback((value: string) => {
    setSlug(value)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const resetForm = useCallback(() => {
    setTitle(page.title)
    setSlug(page.slug)
    setSlugLoading(false)
    setSlugTouched(false)
    setFieldErrors({})
    setLoading(false)
  }, [page.title, page.slug])

  const buildRenamePreview = useCallback(async () => {
    if (!enableLinkRefactor) {
      return getSyntheticRenamePreview({
        page,
        newSlug: slug,
        parentPath,
      })
    }

    return await previewPageRefactor(page.id, {
      kind: 'rename',
      title,
      slug,
    })
  }, [enableLinkRefactor, page, parentPath, slug, title])

  const handleRename = useCallback(async (): Promise<boolean> => {
    if (!title || !slug || isRenameDisabled) return false
    if (slugLoading && !slugTouched) {
      toast.warning('Please wait until the slug is fully generated.')
      return false
    }

    setLoading(true)
    setFieldErrors({})
    try {
      const preview = await buildRenamePreview()
      const rewriteLinks = await (enableLinkRefactor
        ? confirmPageRefactor(preview, { allowSkipRewrite: true })
        : Promise.resolve(false))

      if (enableLinkRefactor && rewriteLinks === null) return false

      const fullPage = await getPageByPath(page.path)

      if (enableLinkRefactor) {
        await applyPageRefactor(page.id, {
          kind: 'rename',
          version: page.version,
          title,
          slug,
          content: fullPage.content,
          rewriteLinks: rewriteLinks as boolean,
        })
      } else {
        await updatePage(
          page.id,
          page.version,
          title,
          slug,
          fullPage.content,
          fullPage.tags ?? [],
          fullPage.properties ?? {},
        )
      }

      await refreshAfterPageRefactor({
        preview,
        currentPath: location.pathname,
        navigate,
      })
      await reloadTree()
      toast.success(`${itemLabel} renamed`)
      return true
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, `Error renaming ${itemLabelLower}`)
      return false
    } finally {
      setLoading(false)
    }
  }, [
    title,
    slug,
    isRenameDisabled,
    slugLoading,
    slugTouched,
    buildRenamePreview,
    enableLinkRefactor,
    page.id,
    page.path,
    page.version,
    location.pathname,
    navigate,
    reloadTree,
    itemLabel,
    itemLabelLower,
  ])

  const buttons = useMemo<BaseDialogConfirmButton[]>(
    () => [
      {
        label: loading ? 'Renaming...' : `Rename ${itemLabel}`,
        actionType: 'confirm',
        autoFocus: true,
        loading,
        disabled: isRenameDisabled,
        variant: 'default',
      },
    ],
    [itemLabel, loading, isRenameDisabled],
  )

  return (
    <BaseDialog
      dialogType={DIALOG_RENAME_PAGE}
      dialogTitle={`Rename ${itemLabel}`}
      dialogDescription={`Rename this ${itemLabelLower} and optionally update its slug`}
      onClose={resetForm}
      onConfirm={async () => await handleRename()}
      testidPrefix="rename-page-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        autoFocus: false,
        disabled: loading,
      }}
      buttons={buttons}
    >
      <div className="page-dialog__fields">
        <div className="page-dialog__title-row">
          <FormInput
            autoFocus
            label="Title"
            value={title}
            onChange={(value) => handleTitleChange(value)}
            testid="rename-page-title-input"
            placeholder={`${itemLabel} title`}
            error={fieldErrors.title}
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="page-dialog__date-btn"
            title="Set title to today"
            onClick={() =>
              handleTitleChange(new Date().toISOString().slice(0, 10))
            }
          >
            <CalendarDays size={15} />
          </Button>
        </div>
        <SlugInputWithSuggestion
          title={title}
          slug={slug}
          testid="rename-page-slug-input"
          parentId={page.parentId || ''}
          currentId={page.id}
          initialTitle={page.title}
          onSlugChange={handleSlugChange}
          onSlugTouchedChange={setSlugTouched}
          onSlugLoadingChange={setSlugLoading}
          error={fieldErrors.slug}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
      </div>
      <span className="dialog__path" data-testid="rename-page-path-display">
        Path: {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
