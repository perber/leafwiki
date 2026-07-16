import { NODE_KIND_PAGE, type Page } from '@/lib/api/pages'
import { useTranslation } from 'react-i18next'

export function useItemLabels(kind: Page['kind'] | string = NODE_KIND_PAGE) {
  const { t } = useTranslation('page')
  const isPage = kind === NODE_KIND_PAGE

  return {
    item: isPage ? t('kinds.page') : t('kinds.section'),
    itemCapitalized: isPage
      ? t('kinds.pageCapitalized')
      : t('kinds.sectionCapitalized'),
  }
}
