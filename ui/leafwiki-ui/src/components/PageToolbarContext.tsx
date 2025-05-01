import { createContext } from 'react'

type ToolbarContent = React.ReactNode

export const PageToolbarContext = createContext<{
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
