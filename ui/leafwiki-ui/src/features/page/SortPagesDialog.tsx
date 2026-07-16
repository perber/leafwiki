// components/page/SortPagesDialog.tsx
import BaseDialog from '@/components/BaseDialog'
import { Button } from '@/components/ui/button'
import { PageNode, sortPages } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_SORT_PAGES } from '@/lib/registries'
import { useTreeStore } from '@/stores/tree'
import {
  DndContext,
  DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ArrowDown, ArrowUp, GripVertical } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useItemLabels } from '@/lib/useItemLabels'

function SortableItem({
  id,
  title,
  index,
  total,
  onMove,
  dragAriaLabel,
}: {
  id: string
  title: string
  index: number
  total: number
  onMove: (index: number, direction: -1 | 1) => void
  dragAriaLabel: string
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <li
      ref={setNodeRef}
      style={style}
      className="sort-pages-dialog__item"
      data-testid={`sort-page-item-${id}`}
    >
      <button
        className="sort-pages-dialog__drag-handle"
        aria-label={dragAriaLabel}
        {...attributes}
        {...listeners}
      >
        <GripVertical size={14} />
      </button>
      <span
        className="sort-pages-dialog__item-title"
        data-testid={`sort-page-title-${id}`}
      >
        {title}
      </span>
      <div className="sort-pages-dialog__item-actions">
        <Button
          variant="ghost"
          size="sm"
          data-testid={`move-up-button-${id}`}
          onClick={() => onMove(index, -1)}
          disabled={index === 0}
        >
          <ArrowUp size={14} />
        </Button>
        <Button
          data-testid={`move-down-button-${id}`}
          variant="ghost"
          size="sm"
          onClick={() => onMove(index, 1)}
          disabled={index === total - 1}
        >
          <ArrowDown size={14} />
        </Button>
      </div>
    </li>
  )
}

export function SortPagesDialog({ parent }: { parent: PageNode }) {
  const { t } = useTranslation('page')
  const { item, itemCapitalized } = useItemLabels(parent.kind)
  const [order, setOrder] = useState(parent.children?.map((c) => c.id) || [])
  const [loading, setLoading] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})
  const reloadTree = useTreeStore((s) => s.reloadTree)

  const nodeMap = useMemo(
    () => new Map(parent.children?.map((c) => [c.id, c]) ?? []),
    [parent.children],
  )

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  )

  useEffect(() => {
    if (!parent) {
      setOrder([])
      return
    }
    setOrder(parent.children?.map((c) => c.id) || [])
  }, [parent])

  const move = (index: number, direction: -1 | 1) => {
    const newOrder = [...order]
    const target = index + direction
    if (target < 0 || target >= newOrder.length) return
    const temp = newOrder[index]
    newOrder[index] = newOrder[target]
    newOrder[target] = temp
    setOrder(newOrder)
  }

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (over && active.id !== over.id) {
      setOrder((prev) => {
        const oldIndex = prev.indexOf(active.id as string)
        const newIndex = prev.indexOf(over.id as string)
        return arrayMove(prev, oldIndex, newIndex)
      })
    }
  }

  const sortAlphabetically = (direction: 'asc' | 'desc') => {
    setOrder((prev) =>
      [...prev].sort((a, b) => {
        const titleA = nodeMap.get(a)?.title ?? ''
        const titleB = nodeMap.get(b)?.title ?? ''
        return direction === 'asc'
          ? titleA.localeCompare(titleB)
          : titleB.localeCompare(titleA)
      }),
    )
  }

  const handleSave = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await sortPages(parent.id, order)
      await reloadTree()
      toast.success(t('toast.sorted', { item: itemCapitalized }))
      return true
    } catch (err) {
      console.warn(err)
      handleFieldErrors(
        err,
        setFieldErrors,
        t('toast.sortError', { item }),
      )
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_SORT_PAGES}
      testidPrefix="sort-pages-dialog"
      dialogTitle={t('sort.title', { item: itemCapitalized })}
      dialogDescription={t('sort.description')}
      onClose={() => true}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          return await handleSave()
        }
        return false
      }}
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        autoFocus: false,
        disabled: loading,
      }}
      buttons={[
        {
          label: t('actions.save'),
          actionType: 'confirm',
          disabled: loading,
          variant: 'default',
          autoFocus: true,
        },
      ]}
    >
      <div className="sort-pages-dialog__toolbar">
        <span className="sort-pages-dialog__toolbar-label">
          {t('sort.sortAlphabetically')}
        </span>
        <Button
          variant="outline"
          size="sm"
          data-testid="sort-az-button"
          onClick={() => sortAlphabetically('asc')}
        >
          {t('sort.asc')}
        </Button>
        <Button
          variant="outline"
          size="sm"
          data-testid="sort-za-button"
          onClick={() => sortAlphabetically('desc')}
        >
          {t('sort.desc')}
        </Button>
      </div>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext items={order} strategy={verticalListSortingStrategy}>
          <ul
            className="custom-scrollbar sort-pages-dialog__list"
            style={{
              maxHeight: '400px',
              height: '400px',
              overflowY: 'auto',
            }}
          >
            {order.map((id, i) => {
              const node = nodeMap.get(id)
              if (!node) return null
              return (
                <SortableItem
                  key={id}
                  id={id}
                  title={node.title}
                  index={i}
                  total={order.length}
                  onMove={move}
                  dragAriaLabel={t('sort.dragAriaLabel')}
                />
              )
            })}
          </ul>
        </SortableContext>
      </DndContext>
    </BaseDialog>
  )
}
