import Breadcrumbs from '@/components/Breadcrumbs'
import { usePageToolbar } from '@/components/PageToolbarContext'
import UserToolbar from '@/components/UserToolbar'
import { AnimatePresence, motion } from 'framer-motion'
import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import Sidebar from './Sidebar'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { content, titleBar } = usePageToolbar()
  const location = useLocation()
  const [isEditor, setIsEditor] = useState(location.pathname.startsWith('/e/'))

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setIsEditor(location.pathname.startsWith('/e/'))
    })
    return () => cancelAnimationFrame(frame)
  }, [location.pathname])

  return (
    <div className="h-screen w-full relative overflow-y-auto bg-gray-50 font-sans text-gray-900">

      <motion.aside
        className="fixed left-0 top-0 bottom-0 z-20 h-full w-96 border-r border-gray-200 bg-white p-4 shadow-md overflow-y-auto"
        animate={{
          x: isEditor ? '-100%' : '0%',
          opacity: isEditor ? 0 : 1,
        }}
        transition={{ duration: 0.1, ease: 'easeInOut' }}
        style={{ willChange: 'transform, opacity' }}
      >
        <Sidebar />
      </motion.aside>

      {/* Main-Content */}
      <motion.div
        className="absolute inset-0 flex flex-col z-10"
        animate={{
          width: isEditor ? '100%' : 'calc(100% - 384px)',
          x: isEditor ? 0 : 384, // ≈ Sidebar-Offset / subtile slide*/
        }}

        transition={{ duration: 0.1, ease: 'easeInOut' }}
        style={{ willChange: 'transform' }}
      >
        <header className="border-b bg-white p-4 shadow-sm min-h-[85px]">
          <div className="flex items-center justify-between h-full">
            <div className="flex items-center gap-2">
              <Breadcrumbs />
            </div>
            <AnimatePresence mode="wait">
              <motion.div
                key={Math.random()}
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 4 }}
                transition={{ duration: 0.2 }}
                className="flex items-center gap-2"
              >
                {titleBar && (
                  <div className="flex flex-1 justify-center items-center">
                    {titleBar}
                  </div>
                )}
              </motion.div>
            </AnimatePresence>

            <div className="flex items-center gap-4">
              <AnimatePresence mode="wait">
                <motion.div
                  key={content?.key || Math.random()}
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 4 }}
                  transition={{ duration: 0.2 }}
                  className="flex items-center gap-2"
                >
                  {content}
                </motion.div>
              </AnimatePresence>
              <UserToolbar />
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </motion.div>
    </div>
  )
}
