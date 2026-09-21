import { Checkbox } from '@/components/ui/checkbox'
import { mapApiError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'

export default function TocDisplaySettings() {
  const { t } = useTranslation('tocDisplay')
  useSetTitle({ title: t('pageTitle') })

  const alwaysShowToc = useConfigStore((s) => s.alwaysShowToc)
  const setAlwaysShowToc = useConfigStore((s) => s.setAlwaysShowToc)

  const [saving, setSaving] = useState(false)

  const apply = async (checked: boolean) => {
    setSaving(true)
    try {
      await setAlwaysShowToc(checked)
      toast.success(t('savedToast'))
    } catch (err) {
      toast.error(mapApiError(err, t('updateError')).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="settings" data-testid="toc-display-settings">
      <h1 className="settings__title">{t('pageTitle')}</h1>
      <div className="settings__section">
        <h2 className="settings__section-title">{t('sectionTitle')}</h2>
        <p className="settings__section-description">
          {t('sectionDescription')}
        </p>

        <div className="settings__field">
          <label className="settings__checkbox-label">
            <Checkbox
              data-testid="toc-display-checkbox"
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
    </div>
  )
}
