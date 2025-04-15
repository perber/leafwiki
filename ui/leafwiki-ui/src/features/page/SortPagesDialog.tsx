// components/page/SortPagesDialog.tsx
import { FormActions } from '@/components/FormActions'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PageNode, sortPages } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { ArrowDown, ArrowUp } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

export function SortPagesDialog({ parent }: { parent: PageNode }) {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === 'sort')

  // State to manage the order of the pages
  const [order, setOrder] = useState(parent.children.map((c) => c.id))

  // Loading state
  const [loading, setLoading] = useState(false)
  const [_, setFieldErrors] = useState<Record<string, string>>({})

  // Reload tree state from zustand store
  const reloadTree = useTreeStore((s) => s.reloadTree)

  useEffect(() => {
    if (!parent) {
      setOrder([])
      return
    }
    setOrder(parent.children.map((c) => c.id))
  }, [parent])

  const onOpenChangeDialog = (open: boolean) => {
    if (!open) {
      closeDialog()
    }
  }

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
      closeDialog()
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error moving page')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChangeDialog}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Sort Pages</DialogTitle>
          <DialogDescription>
            Sort the pages by clicking the arrows. The order will be saved after
            you click "Save".
          </DialogDescription>
        </DialogHeader>

        <ul
          className="space-y-2"
          style={{
            maxHeight: '400px',
            height: '400px',
            overflowY: 'auto',
          }}
        >
          {order.map((id, i) => {
            const node = parent.children.find((c) => c.id === id)
            if (!node) return null
            return (
              <li
                key={id}
                className="flex items-center justify-between rounded-lg border bg-white px-3 py-2 transition hover:shadow-sm"
              >
                <span className="truncate text-sm text-gray-800">
                  {node.title}
                </span>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => move(i, -1)}
                    disabled={i === 0}
                  >
                    <ArrowUp size={14} />
                  </Button>
                  <Button
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
        <DialogFooter>
          <div className="mt-4 flex justify-end">
            <FormActions
              onCancel={() => closeDialog()}
              onSave={handleSave}
              saveLabel={loading ? 'Saving...' : 'Save'}
              disabled={loading}
              loading={loading}
            />
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
