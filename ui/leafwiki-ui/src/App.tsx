import { createLeafWikiRouter } from '@/features/router/router'
import { useBootstrapAuth } from '@/lib/bootstrapAuth'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useBrandingStore } from '@/stores/branding'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import useApplyDesignMode from '@/useApplyDesignMode'
import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'

function App() {
  const configHasLoaded = useConfigStore((s) => s.hasLoaded)
  const loadConfig = useConfigStore((s) => s.loadConfig)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const loadBranding = useBrandingStore((s) => s.loadBranding)

  // bootstrap authentication on app start -> session store
  useBootstrapAuth(configHasLoaded && !authDisabled)
  const isLoggedIn = useSessionStore((s) => !!s.user)
  const isRefreshing = useSessionStore((s) => s.isRefreshing)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()
  useEffect(() => {
    // Load branding configuration
    loadBranding()
    loadConfig()
  }, [loadConfig, loadBranding])

  const router = useMemo(
    () => createLeafWikiRouter(isReadOnlyViewer, authDisabled),
    [isReadOnlyViewer, authDisabled],
  )

  if (!configHasLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

  if (isRefreshing && !authDisabled) {
    return null // avoid router flicker before bootstrapping finished
  }

  return (
    <>
      <Toaster richColors position="bottom-right" />
      <RouterProvider router={router} />
    </>
  )
}

export default App
