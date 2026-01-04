import { DialogManager } from '@/components/DialogManager'
import { HotKeyHandler } from '@/components/HotKeyHandler'
import { Button } from '@/components/ui/button'
import { TooltipProvider } from '@/components/ui/tooltip'
import UserToolbar from '@/components/UserToolbar'
import DesignToggle from '@/features/designtoggle/DesignToggle'
import { EditorTitleBar } from '@/features/editor/EditorTitleBar'
import Progressbar from '@/features/progressbar/Progressbar'
import Sidebar from '@/features/sidebar/Sidebar'
import { Toolbar } from '@/features/toolbar/Toolbar'
import { useAppMode } from '@/lib/useAppMode'
import { useAutoCloseSidebarOnMobile } from '@/lib/useAutoCloseSidebarOnMobile'
import { useIsMobile } from '@/lib/useIsMobile'
import {
  MAX_SIDEBAR_WIDTH,
  MIN_SIDEBAR_WIDTH,
  useSidebarStore,
} from '@/stores/sidebar'
import { MenuIcon } from 'lucide-react'
import React, { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'

export const MOBILE_SIDEBAR_WIDTH = 320

export default function AppLayout({ children }: { children: React.ReactNode }) {
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

  useLayoutEffect(() => {
    // on initial load, close sidebar on mobile
    console.log('isMobile', isMobile)
    if (isMobile) setSidebarVisible(false)
  }, [isMobile, setSidebarVisible])

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
    ? 'custom-scrollbar app-layout__main-content-area-viewer'
    : 'app-layout__main-content-area-editor'

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
      <Progressbar />
      <HotKeyHandler />
      <DialogManager />
      {/* Header */}
      <header className="app-layout__header">
        <div className="app-layout__header-inner">
          <div className="app-layout__sidebar-toggle-container">
            {/* Sidebar Toggle Button */}
            <Button
              variant={'outline'}
              className="app-layout__sidebar-toggle-button"
              onClick={() => setSidebarVisible(!sidebarVisible)}
              aria-label="Toggle Sidebar"
              aria-expanded={sidebarVisible}
              data-testid="sidebar-toggle-button"
            >
              <MenuIcon className="app-layout__sidebar-toggle-button-icon" />
            </Button>
          </div>
          {/* Left side: Logo and Title */}
          <div className="app-layout__logo-n-title">
            <h2>
              <Link to="/">
                🌿 <span className="max-md:hidden">LeafWiki</span>
              </Link>
            </h2>
          </div>
          <div className="app-layout__editor-title-bar-container">
            <EditorTitleBar />
          </div>
          <div className="app-layout__editor-toolbar-container">
            <DesignToggle />
            <Toolbar />
            <UserToolbar />
          </div>
        </div>
      </header>
      <div className="app-layout__header-spacer" />
      <div className="app-layout__content-wrapper">
        <div
          ref={sidebarContainerRef}
          id="sidebar-container"
          className={
            'app-layout__sidebar-container ' +
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
              className="app-layout__sidebar-resizer"
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
                  'app-layout__sidebar-resize-handle ' +
                  (hoveringResize || resizing
                    ? 'app-layout__sidebar-resize-handle-hover'
                    : 'app-layout__sidebar-resize-handle-default')
                }
              />
            </div>
          )}
          <Sidebar />
        </div>

        {/* Overlay for mobile sidebar */}
        {isMobile && sidebarVisible && (
          <div className="app-layout__sidebar-overlay-mobile" />
        )}
        {/* Main content area */}
        <main
          className={`${mainContainerStyle} app-layout__main-content-area`}
          id="scroll-container"
        >
          {children}
        </main>
      </div>
    </TooltipProvider>
  )
}
