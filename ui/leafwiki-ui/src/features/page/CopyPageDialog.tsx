import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { copyPage, NODE_KIND_PAGE, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_COPY_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/routePath'
import { useItemLabels } from '@/lib/useItemLabels'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type CopyPageSource = Pick<PageNode, 'id' | 'title' | 'kind'>

export function CopyPageDialog({ sourcePage }: { sourcePage: CopyPageSource }) {
  const { t } = useTranslation('page')
  const { t: tCommon } = useTranslation('common')
  const { item, itemCapitalized } = useItemLabels(sourcePage.kind)
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
      await copyPage(sourcePage.id, targetParentID, title, slug)
      toast.success(t('toast.copied', { item: itemCapitalized }))
      await reloadTree()
      if (redirect) {
        const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
        navigate(buildEditUrl(fullPath))
      }
      resetForm()
      return true
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, t('toast.copyError', { item }))
      return false
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (sourcePage && sourcePage.title) {
      setTitle(t('copy.defaultTitlePrefix', { title: sourcePage.title }))
    }
  }, [sourcePage, t])

  if (!sourcePage) return null

  if (!tree) return null

  const copyAndEditLabel =
    sourcePage.kind === NODE_KIND_PAGE
      ? t('copy.copyAndEditPage')
      : t('copy.copyAndEditSection')

  return (
    <BaseDialog
      dialogTitle={t('copy.title', { item: itemCapitalized })}
      dialogDescription={t('copy.description', { item })}
      dialogType={DIALOG_COPY_PAGE}
      onClose={handleCancel}
      onConfirm={async (): Promise<boolean> => {
        return await handleCopy(true)
      }}
      testidPrefix="copy-page-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading ? tCommon('actions.copying') : copyAndEditLabel,
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
        label={t('createPage.titleLabel')}
        value={title}
        onChange={(val) => {
          handleTitleChange(val)
        }}
        placeholder={t('editMetadata.titlePlaceholder', {
          item: itemCapitalized,
        })}
        error={fieldErrors.title}
        allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
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
        allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
      />
      <PageSelect pageID={targetParentID} onChange={setTargetParentID} />
      <span className="dialog__path">
        {t('createPage.pathPrefix')} {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
