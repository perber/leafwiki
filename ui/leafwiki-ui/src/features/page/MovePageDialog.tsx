import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { movePage, PageNode } from '@/lib/api'
import { useTreeStore } from '@/stores/tree'
import { Move } from 'lucide-react'
import { JSX, useMemo, useState } from 'react'

export function MovePageDialog({ pageId }: { pageId: string }) {
  const { tree, reloadTree } = useTreeStore()
  const [open, setOpen] = useState(false)

  const parentId = useMemo(() => {
    const findParent = (node: any): string | null => {
      for (const child of node.children || []) {
        if (child.id === pageId) return node.id
        const found = findParent(child)
        if (found) return found
      }
      return null
    }

    if (!tree) return null
    return findParent(tree)
  }, [tree, pageId])

  if (!tree) return null

  if (!parentId) return null

  const [newParentId, setNewParentId] = useState<string>(parentId)

  const handleMove = async () => {
    if (!newParentId || newParentId === parentId) return
    await movePage(pageId, newParentId)
    await reloadTree()
    setOpen(false)
  }

  const renderOptions = (node: PageNode, depth = 1): JSX.Element[] => {
    const indent = '—'.repeat(depth)
    const options = [
      <SelectItem key={node.id} value={node.id}>
        {indent} {node.title}
      </SelectItem>,
    ]

    if (node.children?.length) {
      node.children.forEach((child) => {
        options.push(...renderOptions(child, depth + 1))
      })
    }

    return options
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <div className="relative group flex mr-2">
          <button onClick={() => setOpen(true)}>
            <Move
              size={20}
              className="cursor-pointer text-gray-500 hover:text-gray-800"
            />
          </button>
          <div className="absolute left-0 hidden w-max px-2 py-1 text-xs text-white bg-gray-700 rounded group-hover:block bottom-full mb-2">
            Move page
          </div>
        </div>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Move Page</DialogTitle>
        </DialogHeader>
        <DialogDescription>Select a new parent for this page</DialogDescription>
        <Select value={newParentId} onValueChange={setNewParentId}>
          <SelectTrigger>
            <SelectValue placeholder="Select new parent..." />
          </SelectTrigger>
          <SelectContent>
            <SelectItem key="root" value="root">
              ⬆️ Top Level
            </SelectItem>
            {tree && tree.children.map((child) => renderOptions(child))}
          </SelectContent>
        </Select>

        <div className="mt-4 flex justify-end">
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            className="mr-2"
          >
            Cancel
          </Button>
          <Button onClick={handleMove} disabled={newParentId === parentId}>
            Confirm Move
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
