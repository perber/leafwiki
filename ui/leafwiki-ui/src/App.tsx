import { ErrorBoundary } from '@/components/ErrorBoundary'
import { createLeafWikiRouter } from '@/features/router/router'
import { useBootstrapAuth } from '@/lib/bootstrapAuth'
import { BASE_PATH } from '@/lib/config'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useFavoritesStore } from '@/stores/favorites'
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
  const enableApiKeyManagement = useConfigStore((s) => s.enableApiKeyManagement)
  const userManagementUrl = useConfigStore((s) => s.userManagementUrl)
  const loginUrl = useConfigStore((s) => s.loginUrl)
  const loadBranding = useBrandingStore((s) => s.loadBranding)
  const lastConfigErrorRef = useRef<string | null>(null)

  // bootstrap authentication on app start -> session store
  useBootstrapAuth(configHasLoaded && !authDisabled)

  const isLoggedIn = useSessionStore((s) => !!s.user)
  const userId = useSessionStore((s) => s.user?.id ?? null)
  const isRefreshing = useSessionStore((s) => s.isRefreshing)
  const isReadOnly = useIsReadOnly()
  const isReadOnlyViewer = isReadOnly && !isLoggedIn
  const loadFavorites = useFavoritesStore((s) => s.loadFavorites)
  const clearFavorites = useFavoritesStore((s) => s.clearFavorites)

  useApplyDesignMode()
  useEffect(() => {
    loadConfig()
  }, [loadConfig])

  // Favorites are per-user server truth — (re)load whenever the logged-in
  // user changes, and clear them on logout so a second user on the same
  // browser never sees the first user's favorites.
  useEffect(() => {
    if (userId) {
      loadFavorites()
    } else {
      clearFavorites()
    }
  }, [userId, loadFavorites, clearFavorites])

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
        enableApiKeyManagement,
        userManagementUrl,
        loginUrl,
        BASE_PATH || undefined,
      ),
    [
      isReadOnlyViewer,
      authDisabled,
      enableRevision,
      enableApiKeyManagement,
      userManagementUrl,
      loginUrl,
    ],
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
