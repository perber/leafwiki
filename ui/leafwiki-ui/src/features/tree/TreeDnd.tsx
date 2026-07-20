import {
  convertPage,
  movePage,
  NODE_KIND_PAGE,
  NODE_KIND_SECTION,
  PageNode,
  sortPages,
} from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import {
  DndContext,
  DragEndEvent,
  DragMoveEvent,
  DragOverlay,
  DragStartEvent,
  Modifier,
  MouseSensor,
  TouchSensor,
  pointerWithin,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import { getEventCoordinates } from '@dnd-kit/utilities'
import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { toast } from 'sonner'
import { TreeDndContext } from './treeDndContext'
import {
  buildOrderedIds,
  collectSubtreeIds,
  DropResolution,
  DropTarget,
  getDropZone,
  resolveDrop,
  ROOT_ID,
} from './treeDndUtils'

const EXPAND_ON_HOVER_DELAY_MS = 600

// Dragging the last child out of a section leaves it empty. That may well
// be intentional (mid-reorganization), so it is never converted back
// automatically — instead a toast offers the conversion as a one-click
// action, mirroring how nesting created the section in the first place.
function offerConvertBackToPage(parentId: string) {
  const { byId } = useTreeStore.getState()
  const parent = byId[parentId]
  if (!parent || parent.kind !== NODE_KIND_SECTION) return
  if ((parent.children?.length ?? 0) > 0) return

  toast(`"${parent.title}" is now an empty section`, {
    action: {
      label: 'Convert back to page',
      onClick: () => {
        const node = useTreeStore.getState().byId[parentId]
        if (!node) return
        convertPage(parentId, NODE_KIND_PAGE, node.version)
          .then(() => useTreeStore.getState().reloadTree({ silent: true }))
          .then(() => toast.success(`"${node.title}" is a page again`))
          .catch((err) => {
            console.warn(err)
            toast.error('Failed to convert section back to page')
          })
      },
    },
  })
}

// Keeps the overlay chip attached to the cursor (12px right of it,
// vertically centered) instead of at the dragged row's original offset.
const followCursor: Modifier = ({
  activatorEvent,
  draggingNodeRect,
  transform,
}) => {
  if (!draggingNodeRect || !activatorEvent) {
    return transform
  }
  const coords = getEventCoordinates(activatorEvent)
  if (!coords) {
    return transform
  }
  return {
    ...transform,
    x: transform.x + coords.x - draggingNodeRect.left + 12,
    y:
      transform.y +
      coords.y -
      draggingNodeRect.top -
      draggingNodeRect.height / 2,
  }
}

function getPointerY(
  activatorEvent: Event,
  delta: { y: number },
): number | null {
  const mouseLike = activatorEvent as MouseEvent
  if (typeof mouseLike.clientY === 'number') {
    return mouseLike.clientY + delta.y
  }
  const touch = (activatorEvent as TouchEvent).touches?.[0]
  if (touch) {
    return touch.clientY + delta.y
  }
  return null
}

// The click that follows a drop must never reach the row links: in Firefox
// it targets the dragged link itself and bypasses React's synthetic
// handlers entirely, so the anchor's default action would do a full-page
// navigation. A native window-level capture listener runs before any
// framework code and independent of React event timing, which is why the
// suppression lives here and not in an onClickCapture prop. Armed for the
// whole drag so it's in place no matter when the browser fires the click;
// one drag produces at most one click, so it disarms itself after the
// first one and a stray armed listener can never swallow more.
function suppressClickCapture(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  window.removeEventListener('click', suppressClickCapture, true)
}

function disarmClickSuppression() {
  window.removeEventListener('click', suppressClickCapture, true)
}

export function TreeDndProvider({
  enabled,
  children,
}: {
  enabled: boolean
  children: React.ReactNode
}) {
  const [activeNode, setActiveNode] = useState<PageNode | null>(null)
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null)
  const [saving, setSaving] = useState(false)
  const subtreeIdsRef = useRef<Set<string>>(new Set())
  const expandTimerRef = useRef<{ nodeId: string; timer: number } | null>(null)
  const disarmClickTimerRef = useRef<number | null>(null)

  const armClickSuppression = useCallback(() => {
    if (disarmClickTimerRef.current) {
      window.clearTimeout(disarmClickTimerRef.current)
      disarmClickTimerRef.current = null
    }
    // Re-adding the same listener/capture pair is a no-op, so this is safe
    // even if a previous drag's listener is still armed.
    window.addEventListener('click', suppressClickCapture, true)
  }, [])

  const scheduleClickSuppressionDisarm = useCallback(() => {
    if (disarmClickTimerRef.current) {
      window.clearTimeout(disarmClickTimerRef.current)
    }
    disarmClickTimerRef.current = window.setTimeout(() => {
      disarmClickSuppression()
      disarmClickTimerRef.current = null
    }, 400)
  }, [])

  useEffect(() => disarmClickSuppression, [])

  const sensors = useSensors(
    // Distance threshold keeps plain clicks navigating; on touch a long
    // press starts the drag so the tree still scrolls normally.
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, {
      activationConstraint: { delay: 250, tolerance: 8 },
    }),
  )

  const clearExpandTimer = () => {
    if (expandTimerRef.current) {
      window.clearTimeout(expandTimerRef.current.timer)
      expandTimerRef.current = null
    }
  }

  const resetDragState = () => {
    setActiveNode(null)
    setDropTarget(null)
    subtreeIdsRef.current = new Set()
    clearExpandTimer()
  }

  const updateDropTarget = (next: DropTarget | null) => {
    setDropTarget((prev) =>
      prev?.nodeId === next?.nodeId && prev?.zone === next?.zone ? prev : next,
    )
  }

  const handleDragStart = (event: DragStartEvent) => {
    const node = event.active.data.current?.node as PageNode | undefined
    if (!node) return
    armClickSuppression()
    setActiveNode(node)
    subtreeIdsRef.current = collectSubtreeIds(node)
  }

  const handleDragMove = (event: DragMoveEvent) => {
    const target = event.over?.data.current?.node as PageNode | undefined

    if (!event.over || !target || subtreeIdsRef.current.has(target.id)) {
      updateDropTarget(null)
      clearExpandTimer()
      return
    }

    const pointerY = getPointerY(event.activatorEvent, event.delta)
    const zone =
      pointerY === null ? null : getDropZone(pointerY, event.over.rect)
    if (!zone) {
      updateDropTarget(null)
      clearExpandTimer()
      return
    }

    updateDropTarget({ nodeId: target.id, zone })

    const { isNodeOpen, openNode } = useTreeStore.getState()
    if (
      zone === 'inside' &&
      target.kind === NODE_KIND_SECTION &&
      !isNodeOpen(target.id)
    ) {
      if (expandTimerRef.current?.nodeId !== target.id) {
        clearExpandTimer()
        const timer = window.setTimeout(() => {
          openNode(target.id)
          expandTimerRef.current = null
        }, EXPAND_ON_HOVER_DELAY_MS)
        expandTimerRef.current = { nodeId: target.id, timer }
      }
    } else {
      clearExpandTimer()
    }
  }

  const performDrop = async (dragged: PageNode, resolution: DropResolution) => {
    const { byId, moveNodeLocally, reloadTree, openNode } =
      useTreeStore.getState()
    const currentParentId = dragged.parentId ?? ROOT_ID
    const sameParent = resolution.parentId === currentParentId

    if (sameParent) {
      const siblings = byId[resolution.parentId]?.children ?? []
      const orderedIds = buildOrderedIds(siblings, dragged.id, resolution.index)
      const unchanged = orderedIds.every((id, i) => siblings[i]?.id === id)
      if (unchanged) return

      moveNodeLocally(dragged.id, resolution.parentId, resolution.index)
      setSaving(true)
      try {
        await sortPages(resolution.parentId, orderedIds)
        await reloadTree({ silent: true })
      } catch (err) {
        console.warn(err)
        toast.error('Failed to reorder pages')
        await reloadTree({ silent: true })
      } finally {
        setSaving(false)
      }
      return
    }

    moveNodeLocally(dragged.id, resolution.parentId, resolution.index)
    if (resolution.parentId !== ROOT_ID) {
      openNode(resolution.parentId)
    }
    setSaving(true)
    try {
      await movePage(
        dragged.id,
        dragged.version,
        resolution.parentId,
        resolution.index,
      )
      await reloadTree({ silent: true })
      if (currentParentId !== ROOT_ID) {
        offerConvertBackToPage(currentParentId)
      }
    } catch (err) {
      console.warn(err)
      toast.error('Failed to move page')
      await reloadTree({ silent: true })
    } finally {
      setSaving(false)
    }
  }

  const handleDragEnd = (event: DragEndEvent) => {
    scheduleClickSuppressionDisarm()

    const dragged = event.active.data.current?.node as PageNode | undefined
    const target = dropTarget
    resetDragState()

    if (!dragged || !target) return

    const { byId, isNodeOpen } = useTreeStore.getState()
    const targetNode = byId[target.nodeId]
    if (!targetNode) return

    const resolution = resolveDrop({
      dragged,
      target: targetNode,
      zone: target.zone,
      byId,
      isNodeOpen,
    })
    if (!resolution) return

    void performDrop(dragged, resolution)
  }

  return (
    <TreeDndContext.Provider
      value={{
        enabled: enabled && !saving,
        activeId: activeNode?.id ?? null,
        dropTarget,
      }}
    >
      <DndContext
        sensors={sensors}
        collisionDetection={pointerWithin}
        autoScroll={{ threshold: { x: 0, y: 0.2 } }}
        onDragStart={handleDragStart}
        onDragMove={handleDragMove}
        onDragEnd={handleDragEnd}
        onDragCancel={() => {
          scheduleClickSuppressionDisarm()
          resetDragState()
        }}
      >
        {children}
        {/* Portaled to <body>: the sidebar panel animates via a CSS
            transform, which would otherwise become the containing block
            for the fixed-positioned overlay and offset it from the
            cursor. */}
        {createPortal(
          <DragOverlay dropAnimation={null} modifiers={[followCursor]}>
            {activeNode ? (
              <div className="tree-dnd__overlay">
                {activeNode.title || 'Untitled Page'}
              </div>
            ) : null}
          </DragOverlay>,
          document.body,
        )}
      </DndContext>
    </TreeDndContext.Provider>
  )
}
