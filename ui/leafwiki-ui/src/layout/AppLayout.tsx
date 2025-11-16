import { DialogManager } from '@/components/DialogManager'
import { Button } from '@/components/ui/button'
import { TooltipProvider } from '@/components/ui/tooltip'
import { usePageToolbar } from '@/components/usePageToolbar'
import UserToolbar from '@/components/UserToolbar'
import Sidebar from '@/features/sidebar/Sidebar'
import { useAppMode } from '@/lib/useAppMode'
import { useAutoCloseSidebarOnMobile } from '@/lib/useAutoCloseSidebarOnMobile'
import { useIsMobile } from '@/lib/useIsMobile'
import {
  MAX_SIDEBAR_WIDTH,
  MIN_SIDEBAR_WIDTH,
  useSidebarStore,
} from '@/stores/sidebar'
import { MenuIcon } from 'lucide-react'
import React, { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'

export const MOBILE_SIDEBAR_WIDTH = 320

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { content, titleBar } = usePageToolbar()
  const appMode = useAppMode()
  const [isEditor, setIsEditor] = useState(appMode === 'edit')

  // store resize handler in onMouseMove, onMouseUp in useRef
  const resizeHandlerRef = useRef<{
    onMouseMove: (e: MouseEvent) => void
    onMouseUp: (e: MouseEvent) => void
  } | null>(null)

  const [resizing, setResizing] = useState(false)
  const [hoveringResize, setHoveringResize] = useState(false)

  const sidebarVisible = useSidebarStore((s) => s.sidebarVisible)
  const setSidebarVisible = useSidebarStore((s) => s.setSidebarVisible)
  const sidebarWidth = useSidebarStore((s) => s.sidebarWidth)
  const setSidebarWidth = useSidebarStore((s) => s.setSidebarWidth)
  const isMobile = useIsMobile()

  useAutoCloseSidebarOnMobile()

  const sidebarContainerRef = useRef<HTMLDivElement | null>(null)
  const liveSidebarWidthRef = useRef(sidebarWidth)

  const handleSidebarResize = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!sidebarVisible || isMobile) return

    e.preventDefault()
    e.stopPropagation()

    const startX = e.clientX
    const startWidth = sidebarWidth

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX

      const viewportWidth = window.innerWidth
      const maxWidth = Math.min(viewportWidth - 320, MAX_SIDEBAR_WIDTH) // min. 320px for main content should remain
      const minWidth = MIN_SIDEBAR_WIDTH

      const nextWidth = Math.min(
        maxWidth,
        Math.max(minWidth, startWidth + delta),
      )
      liveSidebarWidthRef.current = nextWidth

      if (sidebarContainerRef.current) {
        sidebarContainerRef.current.style.width = `${nextWidth}px`
      }
    }

    const onMouseUp = () => {
      setSidebarWidth(liveSidebarWidthRef.current)
      setResizing(false)
      setHoveringResize(false)
      resizeHandlerRef.current = null
    }

    resizeHandlerRef.current = { onMouseMove, onMouseUp }
    setResizing(true)
  }

  useEffect(() => {
    if (!resizing || !resizeHandlerRef.current) return

    const { onMouseMove, onMouseUp } = resizeHandlerRef.current

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)

    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
  }, [resizing])

  // cleanup on unmount
  useEffect(() => {
    return () => {
      if (resizeHandlerRef.current) {
        const { onMouseMove, onMouseUp } = resizeHandlerRef.current
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
      }
    }
  }, [])

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setIsEditor(appMode === 'edit')
    })
    return () => cancelAnimationFrame(frame)
  }, [appMode])

  useEffect(() => {
    liveSidebarWidthRef.current = sidebarWidth
  }, [sidebarWidth])

  let mainContainerStyle = !isEditor
    ? 'overflow-auto p-6 custom-scrollbar'
    : 'overflow-hidden'

  // If on mobile and sidebar is visible, hide overflow to prevent double scrollbars
  if (isMobile && sidebarVisible) {
    mainContainerStyle += ' overflow-hidden'
  }

  const effectiveSidebarWidth = !sidebarVisible
    ? 0
    : isMobile
      ? MOBILE_SIDEBAR_WIDTH
      : sidebarWidth

  return (
    <TooltipProvider delayDuration={300}>
      <DialogManager />
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
              data-testid="sidebar-toggle-button"
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
      <div className="content-wrapper flex h-[calc(100dvh-85px)] transition-all duration-200">
        <div
          ref={sidebarContainerRef}
          id="sidebar-container"
          className={
            'custom-scrollbar relative z-20 box-border h-full overflow-auto bg-white pr-1 max-sm:fixed max-sm:h-[calc(100dvh-85px)]' +
            (resizing ? '' : ' transition-[width] duration-200')
          }
          style={{
            width: effectiveSidebarWidth,
            pointerEvents: sidebarVisible ? 'auto' : 'none',
            marginLeft:
              isMobile && !sidebarVisible
                ? '-4px' /* is used to prevent a border when the sidebar is closed */
                : '',
          }}
        >
          {!isMobile && sidebarVisible && (
            <div
              className="absolute inset-y-0 right-0 flex w-1 cursor-col-resize items-center justify-center"
              onMouseDown={handleSidebarResize}
              onMouseEnter={() => setHoveringResize(true)}
              onMouseLeave={() => {
                if (!resizeHandlerRef.current) setHoveringResize(false)
              }}
              role="separator"
              aria-orientation="vertical"
              aria-label="Resize sidebar"
              data-testid="sidebar-resize-handle"
            >
              <div
                className={
                  'pointer-events-none my-4 h-full w-full transition-colors ' +
                  (hoveringResize || resizing ? 'bg-green-400' : 'bg-gray-300')
                }
              />
            </div>
          )}
          <Sidebar />
        </div>

        {/* Overlay for mobile sidebar */}
        {isMobile && sidebarVisible && (
          <div className="fixed inset-0 top-[85px] z-10 bg-black/50 max-sm:h-[calc(100dvh-85px)]" />
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
