// components/page/SortPagesDialog.tsx
import { FormActions } from '@/components/FormActions'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { PageNode, sortPages } from '@/lib/api'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useTreeStore } from '@/stores/tree'
import { ArrowDown, ArrowUp, List } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

export function SortPagesDialog({ parent }: { parent: PageNode }) {
  const [open, setOpen] = useState(false)
  const [order, setOrder] = useState(parent.children.map((c) => c.id))
  const [loading, setLoading] = useState(false)
  const [_, setFieldErrors] = useState<Record<string, string>>({})

  const reloadTree = useTreeStore((s) => s.reloadTree)

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
      setOpen(false)
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error moving page')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <div className="group relative mr-2 flex">
          <button onClick={() => setOpen(true)}>
            <List
              size={20}
              className="cursor-pointer text-gray-500 hover:text-gray-800"
            />
          </button>
          <div className="absolute bottom-full left-0 mb-2 hidden w-max rounded bg-gray-700 px-2 py-1 text-xs text-white group-hover:block">
            Sort pages
          </div>
        </div>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Sort Pages</DialogTitle>
          <DialogDescription>
            Sort the pages by clicking the arrows. The order will be saved after
            you click "Save".
          </DialogDescription>
        </DialogHeader>

        <ul className="space-y-2">
          {order.map((id, i) => {
            const node = parent.children.find((c) => c.id === id)
            if (!node) return null
            return (
              <li
                key={id}
                className="flex items-center justify-between rounded-lg border px-3 py-2 bg-white hover:shadow-sm transition"
              >
                <span className="truncate text-sm text-gray-800">{node.title}</span>
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

        <div className="mt-4 flex justify-end">
          <FormActions
            onCancel={() => setOpen(false)}
            onSave={handleSave}
            saveLabel={loading ? 'Saving...' : 'Save'}
            disabled={loading}
            loading={loading}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
