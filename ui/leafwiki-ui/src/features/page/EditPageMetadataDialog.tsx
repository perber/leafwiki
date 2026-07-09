import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import i18next from '@/lib/i18n'
import { DIALOG_EDIT_PAGE_METADATA } from '@/lib/registries'
import { useItemLabels } from '@/lib/useItemLabels'
import { useTreeStore } from '@/stores/tree'
import { CalendarDays } from 'lucide-react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SlugInputWithSuggestion } from './SlugInputWithSuggestion'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type EditPageMetadataDialogProps = {
  parentId: string
  currentId?: string
  itemKind?: Page['kind']
  title: string
  slug: string
  onChange: (title: string, slug: string) => void
}

export function EditPageMetadataDialog({
  parentId,
  currentId,
  itemKind = NODE_KIND_PAGE,
  title: propTitle,
  slug: propSlug,
  onChange,
}: EditPageMetadataDialogProps) {
  const { t } = useTranslation('page')
  const { item, itemCapitalized } = useItemLabels(itemKind)
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')

  const [title, setTitle] = useState(propTitle)
  const [slug, setSlug] = useState(propSlug)
  const [slugTouched, setSlugTouched] = useState(false)
  const [slugLoading, setSlugLoading] = useState(false)
  const initialLastSlugTitle = currentId ? propTitle : ''
  const [lastSlugTitle, setLastSlugTitle] = useState(initialLastSlugTitle)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const isSaveDisabled =
    !title ||
    !slug ||
    (!slugTouched && (slugLoading || title !== lastSlugTitle))

  const handleTitleChange = (val: string) => {
    setTitle(val)
    setFieldErrors((prev) => ({ ...prev, title: '' }))
  }

  const handleSlugChange = useCallback((val: string) => {
    setSlug(val)
    setFieldErrors((prev) => ({ ...prev, slug: '' }))
  }, [])

  const resetForm = () => {
    setTitle(propTitle)
    setSlug(propSlug)
    setSlugTouched(false)
    setLastSlugTitle(initialLastSlugTitle)
    setFieldErrors({})
  }

  return (
    <BaseDialog
      dialogType={DIALOG_EDIT_PAGE_METADATA}
      dialogTitle={t('editMetadata.title', { item: itemCapitalized })}
      dialogDescription={t('editMetadata.description', { item })}
      onClose={() => {
        resetForm()
        return true
      }}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          onChange(title, slug)
          return true
        }
        return false
      }}
      cancelButton={{
        label: i18next.t('editPageMetadataDialog.cancelButton', {
          ns: 'editor',
        }),
        variant: 'outline',
        autoFocus: false,
      }}
      buttons={[
        {
          label: i18next.t('editPageMetadataDialog.saveButton', {
            ns: 'editor',
          }),
          actionType: 'confirm',
          disabled: isSaveDisabled,
          variant: 'default',
          autoFocus: true,
        },
      ]}
      testidPrefix="edit-page-metadata-dialog"
    >
      <div className="page-dialog__fields">
        <div className="page-dialog__title-row">
          <FormInput
            autoFocus
            label={i18next.t('editPageMetadataDialog.titleLabel', {
              ns: 'editor',
            })}
            value={title}
            onChange={handleTitleChange}
            placeholder={t('editMetadata.titlePlaceholder', {
              item: itemCapitalized,
            })}
            error={fieldErrors.title}
            testid="edit-page-metadata-dialog-title-input"
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="page-dialog__date-btn"
            title={i18next.t('editPageMetadataDialog.dateTitleTooltip', {
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
          currentId={currentId}
          parentId={parentId}
          initialTitle={propTitle}
          enableSlugSuggestion={true}
          onSlugChange={handleSlugChange}
          onSlugTouchedChange={setSlugTouched}
          onSlugLoadingChange={setSlugLoading}
          onLastSlugTitleChange={setLastSlugTitle}
          error={fieldErrors.slug}
          testid="edit-page-metadata-dialog-slug-input"
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
      </div>

      <span
        className="dialog__path"
        data-testid="edit-page-metadata-dialog-path-display"
      >
        {i18next.t('editPageMetadataDialog.pathPrefix', { ns: 'editor' })}{' '}
        {parentPath !== '' && `${parentPath}/`}
        {slug && `${slug}`}
      </span>
    </BaseDialog>
  )
}
