import BaseDialog, { BaseDialogConfirmButton } from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import { createPage, NODE_KIND_PAGE } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import i18next from '@/lib/i18n'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/routePath'
import { useItemLabels } from '@/lib/useItemLabels'
import { useTreeStore } from '@/stores/tree'
import { CalendarDays } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type AddPageDialogProps = {
  parentId: string
  nodeKind?: 'page' | 'section'
}

export function AddPageDialog({
  parentId,
  nodeKind = NODE_KIND_PAGE,
}: AddPageDialogProps) {
  const { t } = useTranslation('page')
  const { itemCapitalized } = useItemLabels(nodeKind)
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
    async (
      redirect: boolean = true,
      kind?: 'page' | 'section',
    ): Promise<boolean> => {
      if (!kind) kind = NODE_KIND_PAGE
      if (!title) return false

      if (!slug) {
        toast.error(t('toast.slugGenerationFailed'))
        return false
      }

      if (!slugTouched && (slugLoading || title !== lastSlugTitle)) {
        toast.warning(t('toast.slugGenerating'))
        return false
      }

      setLoading(true)
      setFieldErrors({})
      try {
        await createPage({ title, slug, parentId, kind })
        toast.success(t('toast.pageCreated', { item: itemCapitalized }))
        await reloadTree()
        if (redirect) {
          const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
          navigate(buildEditUrl(fullPath))
        }
        return true
      } catch (err: unknown) {
        console.warn(err)
        handleFieldErrors(err, setFieldErrors, t('toast.createError'))
        return false
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
      itemCapitalized,
      t,
    ],
  )

  const handleCancel = useCallback(() => {
    resetForm()
    return true
  }, [resetForm])

  const buttons = useMemo(() => {
    const b: BaseDialogConfirmButton[] = [
      {
        label: t('actions.create'),
        actionType: 'no-redirect',
        autoFocus: true,
        loading,
        disabled: isCreateButtonDisabled,
        variant: nodeKind === NODE_KIND_PAGE ? 'secondary' : 'default',
      },
    ]
    if (nodeKind === NODE_KIND_PAGE) {
      b.push({
        label: t('addPage.createAndEditPage'),
        actionType: 'confirm',
        autoFocus: false,
        loading,
        disabled: isCreateButtonDisabled,
        variant: 'default',
      })
    }
    return b
  }, [isCreateButtonDisabled, loading, nodeKind, t])

  return (
    <BaseDialog
      dialogTitle={
        nodeKind === 'page'
          ? t('addPage.titlePage')
          : t('addPage.titleSection')
      }
      dialogDescription={
        nodeKind === 'page'
          ? t('addPage.descriptionPage')
          : t('addPage.descriptionSection')
      }
      dialogType={DIALOG_ADD_PAGE}
      onClose={handleCancel}
      onConfirm={async (actionType: string): Promise<boolean> => {
        return await handleCreate(actionType !== 'no-redirect', nodeKind)
      }}
      testidPrefix="add-page-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={buttons}
    >
      <div className="page-dialog__fields">
        <div className="page-dialog__title-row">
          <FormInput
            autoFocus={true}
            label={t('createPage.titleLabel')}
            value={title}
            onChange={(val) => {
              handleTitleChange(val)
              setFieldErrors((prev) => ({ ...prev, title: '' }))
            }}
            testid="add-page-title-input"
            placeholder={t('editMetadata.titlePlaceholder', { item: itemCapitalized })}
            error={fieldErrors.title}
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="page-dialog__date-btn"
            title={i18next.t('addPageDialog.dateTitleTooltip', {
              ns: 'editor',
            })}
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
          testid="add-page-slug-input"
          parentId={parentId}
          onSlugChange={handleSlugChange}
          onSlugTouchedChange={setSlugTouched}
          onSlugLoadingChange={setSlugLoading}
          onLastSlugTitleChange={setLastSlugTitle}
          error={fieldErrors.slug}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
      </div>
      <span className="dialog__path" data-testid="add-page-path-display">
        {t('createPage.pathPrefix')} {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
