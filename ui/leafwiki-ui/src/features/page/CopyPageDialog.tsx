import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { copyPage, NODE_KIND_PAGE, PageNode } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_COPY_PAGE } from '@/lib/registries'
import { buildEditUrl } from '@/lib/routePath'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { PageSelect } from './PageSelect'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type CopyPageSource = Pick<PageNode, 'id' | 'title' | 'kind'>

export function CopyPageDialog({ sourcePage }: { sourcePage: CopyPageSource }) {
  const { t } = useTranslation('page')
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
  const itemLabel =
    sourcePage.kind === NODE_KIND_PAGE ? t('common.page') : t('common.section')
  const itemLabelCapitalized =
    sourcePage.kind === NODE_KIND_PAGE
      ? t('common.pageCapitalized')
      : t('common.sectionCapitalized')

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
      toast.error(t('copyDialog.slugNotGenerated'))
      return false
    }

    if (!slugTouched && (slugLoading || title !== lastSlugTitle)) {
      toast.warning(t('copyDialog.slugStillGenerating'))
      return false
    }

    setLoading(true)
    setFieldErrors({})
    try {
      await copyPage(sourcePage.id, targetParentID, title, slug)
      toast.success(t('copyDialog.copiedToast', { item: itemLabelCapitalized }))
      await reloadTree()
      if (redirect) {
        const fullPath = parentPath !== '' ? `${parentPath}/${slug}` : slug
        navigate(buildEditUrl(fullPath))
      }
      resetForm()
      return true
    } catch (err: unknown) {
      console.warn(err)
      handleFieldErrors(
        err,
        setFieldErrors,
        t('copyDialog.copyErrorFallback', { item: itemLabel }),
      )
      return false
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (sourcePage && sourcePage.title) {
      setTitle(t('copyDialog.copyOfTitle', { title: sourcePage.title }))
    }
  }, [sourcePage, t])

  if (!sourcePage) return null

  if (!tree) return null

  return (
    <BaseDialog
      dialogTitle={t('copyDialog.title', { item: itemLabelCapitalized })}
      dialogDescription={t('copyDialog.description', { item: itemLabel })}
      dialogType={DIALOG_COPY_PAGE}
      onClose={handleCancel}
      onConfirm={async (): Promise<boolean> => {
        return await handleCopy(true)
      }}
      testidPrefix="copy-page-dialog"
      cancelButton={{
        label: t('common.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading
            ? t('copyDialog.copying')
            : t('copyDialog.copyAndEdit', { item: itemLabelCapitalized }),
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
        label={t('copyDialog.titleLabel')}
        value={title}
        onChange={(val) => {
          handleTitleChange(val)
        }}
        placeholder={t('copyDialog.titlePlaceholder', {
          item: itemLabelCapitalized,
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
        {t('copyDialog.pathPrefix')} {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
