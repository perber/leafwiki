import Breadcrumbs from '@/components/Breadcrumbs'
import { usePageToolbar } from '@/components/PageToolbarContext'
import UserToolbar from '@/components/UserToolbar'
import { AnimatePresence, motion } from 'framer-motion'
import { useLocation } from 'react-router-dom'
import Sidebar from './Sidebar'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { content } = usePageToolbar()
  const location = useLocation()
  const isEditor = location.pathname.startsWith('/e/')
  return (
    <div className="flex h-screen bg-gray-50 font-sans text-gray-900">
      <motion.div
        animate={{
          width: isEditor ? 0 : '24rem',
          opacity: isEditor ? 0 : 1,
        }}
        transition={{ duration: 0.3, ease: 'easeInOut' }}
        className="overflow-hidden"
        style={{ willChange: 'width, opacity' }}
      >
        <motion.aside
          className="h-screen w-96 border-r border-gray-200 bg-white p-4 shadow-md"
          animate={{
            x: isEditor ? -20 : 0,
            opacity: isEditor ? 0 : 1,
          }}
          transition={{ duration: 0.3, ease: 'easeInOut' }}
        >
          <Sidebar />
        </motion.aside>
      </motion.div>
      <div className="flex flex-1 flex-col">
        <header className="flex items-center justify-between border-b bg-white p-4 shadow-sm">
          <div className="mr-2 flex-1 flex-grow">
            <div className="flex items-center gap-2">
              <Breadcrumbs />
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
            </div>
          </div>
          <UserToolbar />
        </header>
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  )
}
