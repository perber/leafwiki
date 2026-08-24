import { Checkbox } from '@/components/ui/checkbox'
import { useUserSettingsStore } from '@/stores/userSettings'
import { useTranslation } from 'react-i18next'

export function PreferencesPanel() {
  const { t } = useTranslation('settings')
  const autoSave = useUserSettingsStore((s) => s.autoSave)
  const toggleAutoSave = useUserSettingsStore((s) => s.toggleAutoSave)

  return (
    <div className="settings__field" data-testid="preferences-panel">
      <label className="settings__checkbox-label">
        <Checkbox
          data-testid="preferences-autosave-checkbox"
          checked={autoSave}
          onCheckedChange={(checked) => {
            if (!!checked !== autoSave) toggleAutoSave()
          }}
        />
        {t('account.preferences.autoSaveLabel')}
      </label>
      <p className="settings__section-description">
        {t('account.preferences.autoSaveDescription')}
      </p>
    </div>
  )
}
