import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { useTranslation } from 'react-i18next'
import { useSetTitle } from '../../viewer/setTitle'
import { AvatarPanel } from './AvatarPanel'
import { ChangeOwnPasswordPanel } from './ChangeOwnPasswordPanel'
import { PreferencesPanel } from './PreferencesPanel'
import { TotpPanel } from './TotpPanel'

export default function AccountSettings() {
  const { t } = useTranslation('settings')
  const totpAvailable = useConfigStore((s) => s.totpAvailable)
  const totpEnabled = useSessionStore((s) => s.user?.totpEnabled ?? false)

  useSetTitle({ title: t('account.pageTitle') })

  return (
    <div className="settings">
      <h1 className="settings__title">{t('account.pageTitle')}</h1>
      <div className="settings__section">
        <h2 className="settings__section-title">
          {t('account.avatar.sectionTitle')}
        </h2>
        <p className="settings__section-description">
          {t('account.avatar.sectionDescription')}
        </p>
        <AvatarPanel />
      </div>

      <div className="settings__section">
        <h2 className="settings__section-title">
          {t('account.passwordSectionTitle')}
        </h2>
        <p className="settings__section-description">
          {t('account.passwordSectionDescription')}
        </p>
        <ChangeOwnPasswordPanel />
      </div>

      {(totpAvailable || totpEnabled) && (
        <div className="settings__section">
          <h2 className="settings__section-title">
            {t('account.twoFactorSectionTitle')}
          </h2>
          <p className="settings__section-description">
            {t('account.twoFactorSectionDescription')}
          </p>
          <TotpPanel />
        </div>
      )}

      <div className="settings__section">
        <h2 className="settings__section-title">
          {t('account.preferences.sectionTitle')}
        </h2>
        <p className="settings__section-description">
          {t('account.preferences.sectionDescription')}
        </p>
        <PreferencesPanel />
      </div>
    </div>
  )
}
