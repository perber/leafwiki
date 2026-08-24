import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getAvailableLanguages } from '@/lib/i18n'
import { useUserSettingsStore } from '@/stores/userSettings'
import { useTranslation } from 'react-i18next'

export function PreferencesPanel() {
  const { t } = useTranslation('settings')
  const autoSave = useUserSettingsStore((s) => s.autoSave)
  const toggleAutoSave = useUserSettingsStore((s) => s.toggleAutoSave)
  const language = useUserSettingsStore((s) => s.language)
  const setLanguage = useUserSettingsStore((s) => s.setLanguage)

  return (
    <div data-testid="preferences-panel">
      <div className="settings__field">
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

      <div className="settings__field">
        <label className="settings__field-label">
          {t('account.preferences.languageLabel')}
        </label>
        <Select
          value={language}
          onValueChange={(value) => {
            void setLanguage(value)
          }}
        >
          <SelectTrigger data-testid="preferences-language-select">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {getAvailableLanguages().map((lang) => (
              <SelectItem key={lang.code} value={lang.code}>
                {lang.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
