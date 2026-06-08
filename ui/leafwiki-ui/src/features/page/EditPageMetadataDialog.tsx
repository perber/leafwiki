import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import i18next from '@/lib/i18n'
import { DIALOG_EDIT_PAGE_METADATA } from '@/lib/registries'
import { useTreeStore } from '@/stores/tree'
import { useCallback, useState } from 'react'
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
  const parentPath = useTreeStore((s) => s.getPathById(parentId) || '')
  const itemLabel = itemKind === NODE_KIND_PAGE ? 'page' : 'section'
  const itemLabelCapitalized = itemKind === NODE_KIND_PAGE ? 'Page' : 'Section'

  const [title, setTitle] = useState(propTitle)
  const [slug, setSlug] = useState(propSlug)
  const [slugTouched, setSlugTouched] = useState(false)
  const [slugLoading, setSlugLoading] = useState(false)
  // In edit mode (currentId set) the slug is already known — initialise to the
  // current title so the save button is enabled immediately.
  const [lastSlugTitle, setLastSlugTitle] = useState(currentId ? propTitle : '')
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
    setLastSlugTitle(currentId ? propTitle : '')
    setFieldErrors({})
  }

  return (
    <BaseDialog
      dialogType={DIALOG_EDIT_PAGE_METADATA}
      dialogTitle={`Edit ${itemLabel} metadata`}
      dialogDescription={`Change metadata of the ${itemLabel}`}
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
        <FormInput
          autoFocus
          label={i18next.t('editPageMetadataDialog.titleLabel', {
            ns: 'editor',
          })}
          value={title}
          onChange={handleTitleChange}
          placeholder={`${itemLabelCapitalized} title`}
          error={fieldErrors.title}
          testid="edit-page-metadata-dialog-title-input"
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />

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
