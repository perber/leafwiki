import { createLeafWikiRouter } from '@/features/router/router'
import { getConfig } from '@/lib/api/config'
import { useBootstrapAuth } from '@/lib/bootstrapAuth'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { usePublicAccessStore } from '@/stores/publicAccess'
import { useSessionStore } from '@/stores/session'
import useApplyDesignMode from '@/useApplyDesignMode'
import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'

function App() {
  // bootstrap authentication on app start -> session store
  useBootstrapAuth()

  const publicAccessLoaded = usePublicAccessStore((s) => s.loaded)
  const setLoaded = usePublicAccessStore((s) => s.setLoaded)
  const setPublicAccess = usePublicAccessStore((s) => s.setPublicAccess)

  const isLoggedIn = useSessionStore((s) => !!s.user)
  const isRefreshing = useSessionStore((s) => s.isRefreshing)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()

  useEffect(() => {
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
  }, [setPublicAccess, setLoaded])

  const router = useMemo(
    () => createLeafWikiRouter(isReadOnlyViewer),
    [isReadOnlyViewer],
  )

  if (!publicAccessLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

  if (isRefreshing) {
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
