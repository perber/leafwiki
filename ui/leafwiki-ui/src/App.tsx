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

function App() {
  return (
    <BrowserRouter>
      <Toaster richColors position="bottom-right" />
      <Routes>
        {/* Login separat, ohne Layout */}
        <Route path="/login" element={<LoginForm />} />

        {/* Alle anderen Routen im AppLayout */}
        <Route
          path="/*"
          element={
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
          }
        />
      </Routes>
    </BrowserRouter>
  )
}

export default App
