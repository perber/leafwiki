import { DialogManger } from '@/components/DialogManager'
import { Button } from '@/components/ui/button'
import { TooltipProvider } from '@/components/ui/tooltip'
import { usePageToolbar } from '@/components/usePageToolbar'
import UserToolbar from '@/components/UserToolbar'
import Sidebar from '@/features/sidebar/Sidebar'
import { useAutoCloseSidebarOnMobile } from '@/lib/useAutoCloseSidebarOnMobile'
import { useIsMobile } from '@/lib/useIsMobile'
import { useSidebarStore } from '@/stores/sidebar'
import { MenuIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { content, titleBar } = usePageToolbar()
  const location = useLocation()
  const [isEditor, setIsEditor] = useState(location.pathname.startsWith('/e/'))

  const sidebarVisible = useSidebarStore((s) => s.sidebarVisible)
  const setSidebarVisible = useSidebarStore((s) => s.setSidebarVisible)
  const isMobile = useIsMobile()

  useAutoCloseSidebarOnMobile()

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setIsEditor(location.pathname.startsWith('/e/'))
    })
    return () => cancelAnimationFrame(frame)
  }, [location.pathname])

  let mainContainerStyle = !isEditor ? 'overflow-auto p-6' : 'overflow-hidden'

  // If on mobile and sidebar is visible, hide overflow to prevent double scrollbars
  if (isMobile && sidebarVisible) {
    mainContainerStyle += ' overflow-hidden'
  }

  return (
    <TooltipProvider delayDuration={300}>
      <DialogManger />
      {/* Header */}
      <header className="fixed z-50 h-[85px] w-full border-b bg-white p-4 shadow-xs">
        <div className="flex h-full items-center justify-start">
          <div className="flex min-h-full w-6 items-center">
            {/* Sidebar Toggle Button */}
            <Button
              variant={'secondary'}
              className="relative z-20 p-2 text-gray-500 hover:text-gray-800 focus:outline-hidden"
              onClick={() => setSidebarVisible(!sidebarVisible)}
              aria-label="Toggle Sidebar"
              aria-expanded={sidebarVisible}
            >
              <MenuIcon className="h-5 w-5" />
            </Button>
          </div>
          {/* Left side: Logo and Title */}
          <div className="mr-6 ml-6 flex min-h-full items-center gap-2">
            <h2 className="text-xl font-bold">
              <Link to="/">
                🌿 <span className="max-md:hidden">LeafWiki</span>
              </Link>
            </h2>
          </div>
          <div className="flex min-h-full flex-1 items-center justify-center">
            {titleBar}
          </div>
          <div className="flex min-h-full items-center gap-4">
            {content}
            <UserToolbar />
          </div>
        </div>
      </header>
      <div className="space-between-header-and-main h-[85px] w-full" />
      <div className="content-wrapper flex h-[calc(100vh-85px)] transition-all duration-200">
        {/* ml-[-1px] is used to prevent a border when the sidebar is closed */}
        <div
          className={`sidebar-wrapper z-20 ml-[-1px] h-full overflow-auto border-r border-gray-200 bg-white transition-all duration-200 max-sm:fixed max-sm:h-[calc(100vh-85px)] ${
            sidebarVisible ? 'w-96' : 'w-0'
          }`}
        >
          <Sidebar />
        </div>
        {/* Overlay for mobile sidebar */}
        {isMobile && sidebarVisible && (
          <div className="fixed inset-0 top-[85px] z-10 bg-black/50 max-sm:h-[calc(100vh-85px)]" />
        )}
        {/* Main content area */}
        <main
          className={`${mainContainerStyle} flex-1 transition-all duration-200`}
          id="scroll-container"
        >
          {children}
        </main>
      </div>
    </TooltipProvider>
  )
}
