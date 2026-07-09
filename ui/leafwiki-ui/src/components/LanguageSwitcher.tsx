import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getLocales, type LocaleInfo } from '@/lib/api/locales'
import { setLanguage } from '@/lib/i18n'
import { Languages } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

type LanguageSwitcherProps = {
  className?: string
}

export function LanguageSwitcher({ className }: LanguageSwitcherProps) {
  const { i18n, t } = useTranslation('common')
  const [languages, setLanguages] = useState<LocaleInfo[]>([])
  const [changing, setChanging] = useState(false)

  useEffect(() => {
    getLocales().then(setLanguages).catch(() => {})
  }, [])

  if (languages.length <= 1) {
    return null
  }

  const currentLanguage = (i18n.resolvedLanguage ?? i18n.language).split('-')[0]

  const handleChange = async (lang: string) => {
    if (lang.split('-')[0] === currentLanguage) {
      return
    }

    setChanging(true)
    try {
      await setLanguage(lang)
    } finally {
      setChanging(false)
    }
  }

  return (
    <div className={className}>
      <Select
        value={currentLanguage}
        onValueChange={(lang) => void handleChange(lang)}
        disabled={changing}
      >
        <SelectTrigger
          className="language-switcher__trigger h-8 w-auto gap-1.5 border-none bg-transparent px-2 shadow-none"
          aria-label={t('language.label')}
        >
          <Languages className="size-4 shrink-0 opacity-70" />
          <SelectValue />
        </SelectTrigger>
        <SelectContent align="end">
          {languages.map((language) => (
            <SelectItem key={language.code} value={language.code}>
              {language.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
