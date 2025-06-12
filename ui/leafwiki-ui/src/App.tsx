import { useEffect } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { Toaster } from 'sonner'
import './App.css'
import { PageToolbarProvider } from './components/PageToolbarProvider'
import LoginForm from './features/auth/LoginForm'
import RequireAuth from './features/auth/RequireAuth'
import PageEditor from './features/page/PageEditor'
import PageViewer from './features/page/PageViewer'
import RootRedirect from './features/page/RootRedirect'
import UserManagement from './features/users/UserManagement'
import AppLayout from './layout/AppLayout'
import { getConfig } from './lib/api'
import { useIsReadOnly } from './lib/useIsReadOnly'
import { useAuthStore } from './stores/auth'
import { usePublicAccessStore } from './stores/publicAccess'

function App() {
  const publicAccessLoaded = usePublicAccessStore((s) => s.loaded)
  const setPublicAccess = usePublicAccessStore((s) => s.setPublicAccess)

  const isLoggedIn = useAuthStore((s) => !!s.user)
  const isReadOnly = useIsReadOnly()
  const allowViewerOnly = isReadOnly && !isLoggedIn

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
        usePublicAccessStore.getState().setLoaded(true)
      })
  }, [setPublicAccess])

  if (!publicAccessLoaded) return null // Config not loaded yet. Show nothing meanwhile or maybe a loading spinner

  return (
    <BrowserRouter>
      <Toaster richColors position="bottom-right" />
      <Routes>
        <Route path="/login" element={<LoginForm />} />
        <Route
          path="/*"
          element={
            allowViewerOnly ? (
              <PageToolbarProvider>
                <AppLayout>
                  <Routes>
                    <Route path="/" element={<RootRedirect />} />
                    <Route path="*" element={<PageViewer />} />
                  </Routes>
                </AppLayout>
              </PageToolbarProvider>
            ) : (
              <RequireAuth>
                <PageToolbarProvider>
                  <AppLayout>
                    <Routes>
                      <Route path="/users" element={<UserManagement />} />
                      <Route path="/" element={<RootRedirect />} />
                      <Route path="/e/*" element={<PageEditor />} />
                      <Route path="*" element={<PageViewer />} />
                    </Routes>
                  </AppLayout>
                </PageToolbarProvider>
              </RequireAuth>
            )
          }
        />
      </Routes>
    </BrowserRouter>
  )
}

export default App
