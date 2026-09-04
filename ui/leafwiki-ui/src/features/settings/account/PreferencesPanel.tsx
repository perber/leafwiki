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

const DATE_FORMAT_OPTIONS = [
  { value: 'locale', labelKey: 'account.preferences.dateFormat.locale' },
  { value: 'iso', labelKey: 'account.preferences.dateFormat.iso' },
  { value: 'dmy_dot', labelKey: 'account.preferences.dateFormat.dmyDot' },
  { value: 'mdy_slash', labelKey: 'account.preferences.dateFormat.mdySlash' },
  { value: 'dmy_slash', labelKey: 'account.preferences.dateFormat.dmySlash' },
] as const

const TIME_FORMAT_OPTIONS = [
  { value: 'locale', labelKey: 'account.preferences.timeFormat.locale' },
  { value: '24h', labelKey: 'account.preferences.timeFormat.h24' },
  { value: '12h', labelKey: 'account.preferences.timeFormat.h12' },
] as const

export function PreferencesPanel() {
  const { t } = useTranslation('settings')
  const autoSave = useUserSettingsStore((s) => s.autoSave)
  const toggleAutoSave = useUserSettingsStore((s) => s.toggleAutoSave)
  const language = useUserSettingsStore((s) => s.language)
  const setLanguage = useUserSettingsStore((s) => s.setLanguage)
  const dateFormat = useUserSettingsStore((s) => s.dateFormat)
  const setDateFormat = useUserSettingsStore((s) => s.setDateFormat)
  const timeFormat = useUserSettingsStore((s) => s.timeFormat)
  const setTimeFormat = useUserSettingsStore((s) => s.setTimeFormat)

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
        <label
          id="preferences-language-label"
          className="settings__field-label"
        >
          {t('account.preferences.languageLabel')}
        </label>
        <Select
          value={language}
          onValueChange={(value) => {
            void setLanguage(value)
          }}
        >
          <SelectTrigger
            data-testid="preferences-language-select"
            aria-labelledby="preferences-language-label"
          >
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

      <div className="settings__field">
        <label
          id="preferences-dateformat-label"
          className="settings__field-label"
        >
          {t('account.preferences.dateFormatLabel')}
        </label>
        <Select
          value={dateFormat}
          onValueChange={(value) => {
            void setDateFormat(value)
          }}
        >
          <SelectTrigger
            data-testid="preferences-dateformat-select"
            aria-labelledby="preferences-dateformat-label"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {DATE_FORMAT_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {t(option.labelKey)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="settings__field">
        <label
          id="preferences-timeformat-label"
          className="settings__field-label"
        >
          {t('account.preferences.timeFormatLabel')}
        </label>
        <Select
          value={timeFormat}
          onValueChange={(value) => {
            void setTimeFormat(value)
          }}
        >
          <SelectTrigger
            data-testid="preferences-timeformat-select"
            aria-labelledby="preferences-timeformat-label"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TIME_FORMAT_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {t(option.labelKey)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
