import { API_BASE_URL } from '../config'

export type LocaleInfo = {
  code: string
  name: string
}

type LocalesResponse = {
  languages: LocaleInfo[]
}

const fallbackLocales: LocaleInfo[] = [
  { code: 'en', name: 'English' },
  { code: 'ru', name: 'Русский' },
]

export async function getLocales(): Promise<LocaleInfo[]> {
  const res = await fetch(`${API_BASE_URL}/api/locales`)
  if (!res.ok) {
    return fallbackLocales
  }

  const data = (await res.json()) as LocalesResponse
  if (!Array.isArray(data.languages) || data.languages.length === 0) {
    return fallbackLocales
  }

  return data.languages
}
