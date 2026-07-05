import { ErrorBoundary } from '@/components/ErrorBoundary'
import { createLeafWikiRouter } from '@/features/router/router'
import { useBootstrapAuth } from '@/lib/bootstrapAuth'
import { BASE_PATH } from '@/lib/config'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useSessionStore } from '@/stores/session'
import useApplyDesignMode from '@/useApplyDesignMode'
import { Loader2 } from 'lucide-react'
import { Suspense, useEffect, useLayoutEffect, useMemo, useRef } from 'react'
import { RouterProvider } from 'react-router-dom'
import { toast, Toaster } from 'sonner'
import './App.css'
import { useBrandingStore } from './stores/branding'
import { useConfigStore } from './stores/config'

function App() {
  const configHasLoaded = useConfigStore((s) => s.hasLoaded)
  const configError = useConfigStore((s) => s.error)
  const loadConfig = useConfigStore((s) => s.loadConfig)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const enableRevision = useConfigStore((s) => s.enableRevision)
  const userManagementUrl = useConfigStore((s) => s.userManagementUrl)
  const loadBranding = useBrandingStore((s) => s.loadBranding)
  const lastConfigErrorRef = useRef<string | null>(null)

  // bootstrap authentication on app start -> session store
  useBootstrapAuth(configHasLoaded && !authDisabled)

  const isLoggedIn = useSessionStore((s) => !!s.user)
  const isRefreshing = useSessionStore((s) => s.isRefreshing)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn

  useApplyDesignMode()
  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  useLayoutEffect(() => {
    // Load branding configuration
    loadBranding()
  }, [loadBranding])

  useEffect(() => {
    if (!configError) {
      lastConfigErrorRef.current = null
      return
    }

    if (lastConfigErrorRef.current === configError) return

    lastConfigErrorRef.current = configError
    toast.error(configError)
  }, [configError])

  const router = useMemo(
    () =>
      createLeafWikiRouter(
        isReadOnlyViewer,
        authDisabled,
        enableRevision,
        userManagementUrl,
        BASE_PATH || undefined,
      ),
    [isReadOnlyViewer, authDisabled, enableRevision, userManagementUrl],
  )

  return (
    <>
      <Toaster richColors position="bottom-right" />
      {configHasLoaded && !(isRefreshing && !authDisabled) ? (
        <ErrorBoundary>
          <Suspense
            fallback={
              <div className="flex h-screen items-center justify-center">
                <Loader2 className="text-muted-foreground h-8 w-8 animate-spin" />
              </div>
            }
          >
            <RouterProvider router={router} />
          </Suspense>
        </ErrorBoundary>
      ) : null}
    </>
  )
}

export default App
