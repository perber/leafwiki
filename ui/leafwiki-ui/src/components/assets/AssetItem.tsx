import { Button } from '@/components/ui/button'
import { deleteAsset, renameAsset } from '@/lib/api'
import { Check, FileText, Pencil, Trash2, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { AssetPreviewTooltip } from './AssetPreviewTooltip'


const imageExtensions = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg']

type Props = {
    pageId: string
    filename: string
    onReload: () => void
    onInsert: (md: string) => void
}

export function AssetItem({ pageId, filename, onReload, onInsert }: Props) {
    const assetUrl = filename
    const ext = filename.split('.').pop()?.toLowerCase()
    const isImage = imageExtensions.includes(ext ?? '')
    const baseName = filename.split('/').pop() ?? filename

    const [isEditing, setIsEditing] = useState(false)
    const [newName, setNewName] = useState(baseName.replace(/\.[^/.]+$/, ''))

    const handleRename = async () => {
        try {
            const newFilename = `${newName}.${ext}`
            if (newFilename === baseName) {
                setIsEditing(false)
                return
            }

            await renameAsset(pageId, baseName, newFilename)
            toast.success('Asset renamed')
            onReload()
        } catch (err) {
            toast.error('Rename failed')
            console.error('Rename failed', err)
        } finally {
            setIsEditing(false)
        }
    }

    const handleDelete = async () => {
        try {
            await deleteAsset(pageId, baseName)
            toast.success('Asset deleted')
            onReload()
        } catch (err) {
            toast.error('Delete failed')
            console.error('Delete failed', err)
        }
    }

    const handleInsertMarkdown = () => {
        if (!isEditing) {
            const markdown = isImage
                ? `![${newName}](${assetUrl})\n`
                : `[${baseName}](${assetUrl})\n`
            onInsert(markdown)
        }
    }

    return (
        <li
            className="group flex items-center justify-between gap-2 rounded-md px-2 py-1 transition hover:bg-gray-100"
            onDoubleClick={handleInsertMarkdown}
        >
            <div className="flex flex-1 items-center gap-1">
                {isImage ? (
                    <AssetPreviewTooltip url={assetUrl} name={baseName}>
                        <img
                            src={assetUrl}
                            alt={baseName}
                            className="h-10 w-10 rounded border object-cover"
                        />
                    </AssetPreviewTooltip>
                ) : (
                    <AssetPreviewTooltip url={assetUrl} name={baseName}>
                        <div className="flex h-10 w-10 items-center justify-center rounded border bg-gray-100 text-gray-500">
                            <FileText size={18} />
                        </div>
                    </AssetPreviewTooltip>
                )}

                {isEditing ? (
                    <input
                        autoFocus={true}
                        value={newName}
                        onChange={(e) => setNewName(e.target.value)}
                        onKeyDownCapture={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                handleRename()
                            }
                            if (e.key === 'Escape') {
                                e.preventDefault()
                                e.stopPropagation()
                                setIsEditing(false)
                                setNewName(baseName.replace(/\.[^/.]+$/, ''))
                            }
                        }}
                        className="w-full border-b border-gray-300 bg-transparent text-sm text-gray-800 focus:outline-none"
                    />
                ) : (
                    <span className="truncate text-sm text-gray-800 hover:underline">
                        {baseName}
                    </span>
                )}
            </div>

            {isEditing ? (
                <>
                    <Button
                        variant="ghost"
                        size="icon"
                        className="text-green-600 hover:text-green-700"
                        onClick={handleRename}
                        title="Save"
                    >
                        <Check size={16} />
                    </Button>
                    <Button
                        variant="ghost"
                        size="icon"
                        className="text-red-600 hover:text-red-600"
                        onClick={() => {
                            setIsEditing(false)
                            setNewName(baseName.replace(/\.[^/.]+$/, ''))
                        }}
                        title="Cancel"
                    >
                        <X size={16} />
                    </Button>
                </>
            ) : (
                <Button
                    variant="ghost"
                    size="icon"
                    className="text-gray-400 hover:text-blue-600"
                    onClick={(e) => {
                        e.stopPropagation()
                        setNewName(baseName.replace(/\.[^/.]+$/, ''))
                        setIsEditing(true)
                    }}
                    title="Rename"
                >
                    <Pencil size={16} />
                </Button>
            )}

            <Button
                variant="ghost"
                size="icon"
                className="text-gray-400 hover:text-red-600"
                onClick={(e) => {
                    e.stopPropagation()
                    handleDelete()
                }}
                title="Delete"
            >
                <Trash2 size={16} />
            </Button>
        </li>
    )
}
