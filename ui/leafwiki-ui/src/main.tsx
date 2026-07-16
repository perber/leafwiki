import './index.css'

import { initI18n } from '@/lib/i18n'
import '@fontsource/inter'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

initI18n().then(async () => {
  const { default: App } = await import('./App.tsx')

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
})
