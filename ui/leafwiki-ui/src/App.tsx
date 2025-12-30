import { createLeafWikiRouter } from '@/features/router/router'
import { useBootstrapAuth } from '@/lib/bootstrapAuth'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useSessionStore } from '@/stores/session'
import useApplyDesignMode from '@/useApplyDesignMode'
import { useEffect, useMemo } from 'react'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'
import { useConfigStore } from './stores/config'

function App() {
  const configHasLoaded = useConfigStore((s) => s.hasLoaded)
  const loadConfig = useConfigStore((s) => s.loadConfig)
  // bootstrap authentication on app start -> session store
  useBootstrapAuth()

  const isLoggedIn = useSessionStore((s) => !!s.user)
  const isRefreshing = useSessionStore((s) => s.isRefreshing)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()
  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  const router = useMemo(
    () => createLeafWikiRouter(isReadOnlyViewer),
    [isReadOnlyViewer],
  )

  if (!configHasLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

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
