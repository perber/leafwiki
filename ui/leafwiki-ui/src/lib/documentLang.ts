// Keeps <html lang> in sync with the active i18next UI language.
//
// index.html ships lang="en"; without this a German UI still claims to be
// English, so screen readers pick the wrong voice and the browser's
// spellcheck in the editor can pick the wrong dictionary (#1731).
//
// Imported for its side effect from src/lib/i18n.ts. Imports are hoisted, so
// this runs before i18next.init(); the initial value arrives through the
// languageChanged event that init() fires.

import i18next from 'i18next'

function applyDocumentLang(lng?: string): void {
  const lang = lng ?? i18next.language
  if (lang) document.documentElement.lang = lang
}

applyDocumentLang()
i18next.on('languageChanged', applyDocumentLang)
