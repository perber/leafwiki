import {
  MouseEvent as ReactMouseEvent,
  useEffect,
  useRef,
  useState,
} from 'react'

const DEFAULT_EDITOR_PANE_WIDTH = 50
export const MIN_EDITOR_PANE_WIDTH = 25
export const MAX_EDITOR_PANE_WIDTH = 75
const EDITOR_PANE_WIDTH_STORAGE_KEY = 'leafwiki-editor-pane-width'

function clampEditorPaneWidth(value: number) {
  return Math.min(MAX_EDITOR_PANE_WIDTH, Math.max(MIN_EDITOR_PANE_WIDTH, value))
}

function getInitialEditorPaneWidth() {
  if (typeof window === 'undefined') return DEFAULT_EDITOR_PANE_WIDTH

  const storedValue = window.localStorage.getItem(EDITOR_PANE_WIDTH_STORAGE_KEY)
  const parsed = Number.parseFloat(storedValue ?? '')

  if (Number.isNaN(parsed)) return DEFAULT_EDITOR_PANE_WIDTH

  return clampEditorPaneWidth(parsed)
}

type Options = {
  isMobile: boolean
  showPreview: boolean
  previewStacked: boolean
}

// useSplitPaneResize owns the editor/preview split: the editor pane width
// (persisted in localStorage, clamped to 25–75%) and the drag on the divider,
// horizontal or vertical depending on previewStacked.
export function useSplitPaneResize({
  isMobile,
  showPreview,
  previewStacked,
}: Options) {
  const desktopSplitRef = useRef<HTMLDivElement | null>(null)
  const liveEditorPaneWidthRef = useRef(DEFAULT_EDITOR_PANE_WIDTH)
  const resizeHandlersRef = useRef<{
    onMouseMove: (event: MouseEvent) => void
    onMouseUp: () => void
  } | null>(null)

  const [editorPaneWidth, setEditorPaneWidth] = useState(
    getInitialEditorPaneWidth,
  )
  const [isResizingSplit, setIsResizingSplit] = useState(false)

  useEffect(() => {
    liveEditorPaneWidthRef.current = editorPaneWidth
  }, [editorPaneWidth])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem(
      EDITOR_PANE_WIDTH_STORAGE_KEY,
      String(editorPaneWidth),
    )
  }, [editorPaneWidth])

  useEffect(() => {
    if (!isResizingSplit || !resizeHandlersRef.current) return

    const { onMouseMove, onMouseUp } = resizeHandlersRef.current
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)

    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }
  }, [isResizingSplit])

  useEffect(
    () => () => {
      if (!resizeHandlersRef.current) return

      const { onMouseMove, onMouseUp } = resizeHandlersRef.current
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    },
    [],
  )

  const handleSplitResize = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (isMobile || !showPreview) return

    event.preventDefault()
    event.stopPropagation()

    const startPosition = previewStacked ? event.clientY : event.clientX
    const startWidth = editorPaneWidth
    const splitRect = desktopSplitRef.current?.getBoundingClientRect()
    const splitSize =
      (previewStacked ? splitRect?.height : splitRect?.width) ||
      (previewStacked ? window.innerHeight : window.innerWidth)

    const onMouseMove = (moveEvent: MouseEvent) => {
      const currentPosition = previewStacked
        ? moveEvent.clientY
        : moveEvent.clientX
      const delta = currentPosition - startPosition
      const nextWidth = clampEditorPaneWidth(
        startWidth + (delta / splitSize) * 100,
      )

      liveEditorPaneWidthRef.current = nextWidth
      setEditorPaneWidth(nextWidth)
    }

    const onMouseUp = () => {
      setEditorPaneWidth(liveEditorPaneWidthRef.current)
      setIsResizingSplit(false)
      resizeHandlersRef.current = null
    }

    resizeHandlersRef.current = { onMouseMove, onMouseUp }
    setIsResizingSplit(true)
  }

  return {
    desktopSplitRef,
    editorPaneWidth,
    isResizingSplit,
    handleSplitResize,
  }
}
