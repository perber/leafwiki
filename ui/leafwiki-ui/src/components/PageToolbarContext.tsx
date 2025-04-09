// src/context/PageToolbarContext.tsx
import { createContext, useContext, useState } from 'react'

type ToolbarContent = React.ReactNode

const PageToolbarContext = createContext<{
  setContent: (content: ToolbarContent) => void
  clear: () => void
  content: ToolbarContent
}>({
  setContent: () => {},
  clear: () => {},
  content: null,
})

export function PageToolbarProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [content, setContent] = useState<ToolbarContent>(null)

  return (
    <PageToolbarContext.Provider
      value={{
        content,
        setContent,
        clear: () => setContent(null),
      }}
    >
      {children}
    </PageToolbarContext.Provider>
  )
}

export function usePageToolbar() {
  return useContext(PageToolbarContext)
}
