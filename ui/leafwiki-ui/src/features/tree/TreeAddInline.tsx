
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { createPage, suggestSlug } from "@/lib/api"
import { useTreeStore } from "@/stores/tree"
import { Plus } from "lucide-react"
import { useState } from "react"

type TreeAddInlineProps = {
    parentId: string
    minimal?: boolean
}

export function TreeAddInline({ parentId, minimal }: TreeAddInlineProps) {

    const [open, setOpen] = useState(false)
    const [title, setTitle] = useState("")
    const [slug, setSlug] = useState("")
    const reloadTree = useTreeStore((s) => s.reloadTree)
    const parentPath = useTreeStore((s) => s.getPathById(parentId) || "")

    const handleTitleChange = async (val: string) => {
        setTitle(val)
        if (!val.trim()) {
            setSlug("")
            return
        }
        try {
            const suggestion = await suggestSlug(parentId, val)
            setSlug(suggestion)
        } catch (err) {
            console.warn(err)
        }
    }

    const handleCreate = async () => {
        if (!title || !slug) return
        await createPage({ title, slug, parentId })
        await reloadTree()
        setOpen(false)
        setTitle("")
        setSlug("")
    }

    return <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
            {minimal ? (
                <button className="flex items-center text-sm text-gray-500 hover:text-gray-800" onClick={() => setOpen(true)}>
                    <Plus className="mr-1 w-4" />
                </button>
            ) : (
                <button className="flex items-center text-sm text-gray-500 hover:text-gray-800" onClick={() => setOpen(true)}>
                    <Plus className="mr-1 w-4" />
                    Create page {parentId}
                </button>
            )}
        </DialogTrigger>
        <DialogContent>
            <DialogHeader>
                <DialogTitle>Create a new page</DialogTitle>
                <DialogDescription>Enter the title of the new page</DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
                <Input placeholder="Title" value={title} onChange={(e) => handleTitleChange(e.target.value)} />
                <Input placeholder="Slug" value={slug} onChange={(e) => setSlug(e.target.value)} />
            </div>
            <span className="text-sm text-gray-500">
                Path: {parentPath !== "" && `${parentPath}/`}{slug && `${slug}`}
            </span>
            <div className="flex justify-end mt-4">
                <Button onClick={handleCreate} disabled={!title || !slug}>
                    Create
                </Button>
            </div>
        </DialogContent>
    </Dialog>
}