import { getConfig } from '@/lib/api/config'
import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'
import { createLeafWikiRouter } from './features/router/router'
import { useIsReadOnly } from './lib/useIsReadOnly'
import { useAuthStore } from './stores/auth'
import { useBrandingStore } from './stores/branding'
import { usePublicAccessStore } from './stores/publicAccess'
import useApplyDesignMode from './useApplyDesignMode'

function App() {
  const publicAccessLoaded = usePublicAccessStore((s) => s.loaded)
  const setLoaded = usePublicAccessStore((s) => s.setLoaded)
  const setPublicAccess = usePublicAccessStore((s) => s.setPublicAccess)

  const loadBranding = useBrandingStore((s) => s.loadBranding)

  const isLoggedIn = useAuthStore((s) => !!s.user)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()

  useEffect(() => {
    // Load branding configuration
    loadBranding()

    getConfig()
      .then((config) => {
        if (!config) {
          throw new Error('Failed to load configuration')
        }
        setPublicAccess(config.publicAccess)
      })
      .catch((error) => {
        console.warn(
          'Error loading configuration: Set public mode to false!',
          error,
        )
        setPublicAccess(false) // Fallback to false if config fails
      })
      .finally(() => {
        setLoaded(true) // Mark public access as loaded
      })
  }, [setPublicAccess, setLoaded, loadBranding])

  const router = useMemo(
    () => createLeafWikiRouter(isReadOnlyViewer),
    [isReadOnlyViewer],
  )

  if (!publicAccessLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

  return (
    <>
      <Toaster richColors position="bottom-right" />
      <RouterProvider router={router} />
    </>
  )
}

export default App
