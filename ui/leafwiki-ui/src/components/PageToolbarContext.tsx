// src/context/PageToolbarContext.tsx
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react'

type ToolbarContent = React.ReactNode

const PageToolbarContext = createContext<{
  setContent: (content: ToolbarContent) => void
  clearContent: () => void
  setTitleBar: (titleBar: ToolbarContent) => void
  clearTitleBar: () => void
  content: ToolbarContent
  titleBar: ToolbarContent
}>({
  setContent: () => {},
  setTitleBar: () => {},
  clearContent: () => {},
  clearTitleBar: () => {},
  content: null,
  titleBar: null,
})

export function PageToolbarProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [content, setContentState] = useState<ToolbarContent>(null)
  const [titleBar, setTitleBarState] = useState<ToolbarContent>(null)

  const setContent = useCallback((c: ToolbarContent) => {
    setContentState(c)
  }, [])

  const clearContent = useCallback(() => {
    setContentState(null)
  }, [])

  const setTitleBar = useCallback((c: ToolbarContent) => {
    setTitleBarState(c)
  }, [])

  const clearTitleBar = useCallback(() => {
    setTitleBarState(null)
  }, [])

  const value = useMemo(() => {
    return {
      content,
      titleBar,
      setContent,
      clearContent,
      setTitleBar,
      clearTitleBar,
    }
  }, [content, setContent, clearContent, setTitleBar, titleBar, clearTitleBar])

  return (
    <PageToolbarContext.Provider value={value}>
      {children}
    </PageToolbarContext.Provider>
  )
}

export function usePageToolbar() {
  return useContext(PageToolbarContext)
}
