import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { mapApiError } from '@/lib/api/errors'
import { CONTENT_WIDTHS, isContentWidth } from '@/lib/contentWidth'
import { useConfigStore } from '@/stores/config'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'

export default function DisplaySettings() {
  const { t } = useTranslation('display')
  useSetTitle({ title: t('pageTitle') })

  const alwaysShowToc = useConfigStore((s) => s.pageDisplay.alwaysShowToc)
  const contentWidth = useConfigStore((s) => s.pageDisplay.contentWidth)
  const updatePageDisplay = useConfigStore((s) => s.updatePageDisplay)

  const [saving, setSaving] = useState(false)
  const [savingWidth, setSavingWidth] = useState(false)

  const apply = async (checked: boolean) => {
    setSaving(true)
    try {
      await updatePageDisplay({ alwaysShowToc: checked })
      toast.success(t('savedToast'))
    } catch (err) {
      toast.error(mapApiError(err, t('updateError')).message)
    } finally {
      setSaving(false)
    }
  }

  const applyWidth = async (value: string) => {
    if (!isContentWidth(value) || value === contentWidth) return
    setSavingWidth(true)
    try {
      await updatePageDisplay({ contentWidth: value })
      toast.success(t('contentWidthSavedToast'))
    } catch (err) {
      toast.error(mapApiError(err, t('contentWidthUpdateError')).message)
    } finally {
      setSavingWidth(false)
    }
  }

  return (
    <div className="settings" data-testid="display-settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>
      <div className="settings__section">
        <h2 className="settings__section-title">{t('sectionTitle')}</h2>
        <p className="settings__section-description">
          {t('sectionDescription')}
        </p>

        <div className="settings__field">
          <label className="settings__checkbox-label">
            <Checkbox
              data-testid="display-toc-checkbox"
              checked={alwaysShowToc}
              disabled={saving}
              onCheckedChange={(checked) => {
                if (!!checked !== alwaysShowToc) void apply(!!checked)
              }}
            />
            {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t('checkboxLabel')}
          </label>
          <p className="settings__section-description">
            {t('checkboxDescription')}
          </p>
        </div>
      </div>

      <div className="settings__section">
        <h2 className="settings__section-title">
          {t('contentWidthSectionTitle')}
        </h2>
        <p className="settings__section-description">
          {t('contentWidthSectionDescription')}
        </p>

        <div className="settings__field">
          <label
            id="display-content-width-label"
            className="settings__field-label"
          >
            {t('contentWidthLabel')}
            {savingWidth && (
              <Loader2 className="ml-2 inline h-4 w-4 animate-spin" />
            )}
          </label>
          <Select
            value={contentWidth}
            disabled={savingWidth}
            onValueChange={(value) => {
              void applyWidth(value)
            }}
          >
            <SelectTrigger
              data-testid="display-content-width-select"
              aria-labelledby="display-content-width-label"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {CONTENT_WIDTHS.map((width) => (
                <SelectItem key={width} value={width}>
                  {t(`contentWidthOptions.${width}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="settings__section-description">
            {t('contentWidthDescription')}
          </p>
        </div>
      </div>
    </div>
  )
}
