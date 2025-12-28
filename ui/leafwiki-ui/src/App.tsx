import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'
import { createLeafWikiRouter } from './features/router/router'
import { useIsReadOnly } from './lib/useIsReadOnly'
import { useAuthStore } from './stores/auth'
import { useBrandingStore } from './stores/branding'
import { useConfigStore } from './stores/config'
import useApplyDesignMode from './useApplyDesignMode'

function App() {
  const configHasLoaded = useConfigStore((s) => s.hasLoaded)
  const loadConfig = useConfigStore((s) => s.loadConfig)

  const loadBranding = useBrandingStore((s) => s.loadBranding)

  const isLoggedIn = useAuthStore((s) => !!s.user)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()
  useEffect(() => {
    // Load branding configuration
    loadBranding()
    loadConfig()
  }, [loadConfig, loadBranding])

  const router = useMemo(
    () => createLeafWikiRouter(isReadOnlyViewer),
    [isReadOnlyViewer],
  )

  if (!configHasLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

  return (
    <>
      <Toaster richColors position="bottom-right" />
      <RouterProvider router={router} />
    </>
  )
}

export default App
