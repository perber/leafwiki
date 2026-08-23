import {
  ListView,
  ListViewHeader,
  ListViewItem,
  ListViewList,
} from '@/components/ListView'
import {
  isSectionVisible,
  settingsSections,
  useSettingsSectionContext,
} from '@/lib/registries/settingsSectionRegistry'
import { Settings } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'

export default function SettingsNav() {
  const { t } = useTranslation('settings')
  const navigate = useNavigate()
  const location = useLocation()
  const ctx = useSettingsSectionContext()

  const visibleSections = settingsSections.filter((section) =>
    isSectionVisible(section, ctx),
  )

  return (
    <ListView
      header={
        <ListViewHeader>
          <Settings size={16} />
          {t('nav.title')}
        </ListViewHeader>
      }
      className="settings-nav"
      testId="settings-nav"
    >
      <ListViewList>
        {visibleSections.map((section) => {
          const Icon = section.icon
          const externalHref = section.externalHref?.(ctx)
          const active = location.pathname.endsWith(`/settings/${section.path}`)

          if (externalHref) {
            return (
              <a
                key={section.id}
                href={externalHref}
                target="_blank"
                rel="noopener noreferrer"
                className="list-view__item"
                data-testid={`settings-nav-item-${section.id}`}
              >
                <Icon size={16} />
                {t(section.labelKey, { ns: section.ns })}
              </a>
            )
          }

          return (
            <ListViewItem
              key={section.id}
              active={active}
              onClick={() => navigate(`/settings/${section.path}`)}
              testId={`settings-nav-item-${section.id}`}
            >
              <Icon size={16} />
              {t(section.labelKey, { ns: section.ns })}
            </ListViewItem>
          )
        })}
      </ListViewList>
    </ListView>
  )
}
