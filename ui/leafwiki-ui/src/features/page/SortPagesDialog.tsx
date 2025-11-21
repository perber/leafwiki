// components/page/SortPagesDialog.tsx
import BaseDialog from '@/components/BaseDialog'
import { Button } from '@/components/ui/button'
import { PageNode, sortPages } from '@/lib/api/pages'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_SORT_PAGES } from '@/lib/registries'
import { useTreeStore } from '@/stores/tree'
import { ArrowDown, ArrowUp } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

export function SortPagesDialog({ parent }: { parent: PageNode }) {
  // State to manage the order of the pages
  const [order, setOrder] = useState(parent.children?.map((c) => c.id) || [])

  // Loading state
  const [loading, setLoading] = useState(false)
  const [, setFieldErrors] = useState<Record<string, string>>({})

  // Reload tree state from zustand store
  const reloadTree = useTreeStore((s) => s.reloadTree)

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

  const handleSave = async () => {
    setLoading(true)
    try {
      await sortPages(parent.id, order)
      await reloadTree()
      toast.success('Pages sorted successfully')
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error moving page')
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_SORT_PAGES}
      dialogTitle="Sort Pages"
      dialogDescription="Sort the pages by clicking the arrows. The order will be saved after you click 'Save'."
      onClose={() => true}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          await handleSave()
          return true
        }
        return false
      }}
      cancelButton={{ label: 'Cancel', variant: 'outline', autoFocus: false }}
      buttons={[
        {
          label: 'Save',
          actionType: 'confirm',
          disabled: loading,
          variant: 'default',
          autoFocus: true,
        },
      ]}
    >
      <ul
        className="custom-scrollbar space-y-2"
        style={{
          maxHeight: '400px',
          height: '400px',
          overflowY: 'auto',
        }}
      >
        {order.map((id, i) => {
          const node = parent.children?.find((c) => c.id === id)
          if (!node) return null
          return (
            <li
              key={id}
              className="flex items-center justify-between rounded-lg border bg-white px-3 py-2 transition hover:shadow-xs"
              data-testid={`sort-page-item-${id}`}
            >
              <span
                className="truncate text-sm text-gray-800"
                data-testid={`sort-page-title-${id}`}
              >
                {node.title}
              </span>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  data-testid={`move-up-button-${id}`}
                  onClick={() => move(i, -1)}
                  disabled={i === 0}
                >
                  <ArrowUp size={14} />
                </Button>
                <Button
                  data-testid={`move-down-button-${id}`}
                  variant="ghost"
                  size="sm"
                  onClick={() => move(i, 1)}
                  disabled={i === order.length - 1}
                >
                  <ArrowDown size={14} />
                </Button>
              </div>
            </li>
          )
        })}
      </ul>
    </BaseDialog>
  )
}
