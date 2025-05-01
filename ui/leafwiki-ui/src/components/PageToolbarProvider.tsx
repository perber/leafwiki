// src/context/PageToolbarContext.tsx
import {
  useCallback,
  useMemo,
  useState
} from 'react'
import { PageToolbarContext } from './PageToolbarContext'

type ToolbarContent = React.ReactNode


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
