// components/page/SortPagesDialog.tsx
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
import { useTreeStore } from '@/stores/tree'
import { ArrowDown, ArrowUp, List } from 'lucide-react'
import { useState } from 'react'

export function SortPagesButton({ parent }: { parent: PageNode }) {
  const [open, setOpen] = useState(false)
  const [order, setOrder] = useState(parent.children.map((c) => c.id))
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
    await sortPages(parent.id, order)
    await reloadTree()
    setOpen(false)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <button className="text-gray-500 hover:text-gray-800">
          <List size={16} />
        </button>
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
                className="flex items-center justify-between rounded border p-2"
              >
                <span className="truncate">{node.title}</span>
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
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            className="mr-2"
          >
            Cancel
          </Button>
          <Button onClick={handleSave}>Save</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
