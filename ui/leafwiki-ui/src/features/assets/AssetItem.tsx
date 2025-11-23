import { Button } from '@/components/ui/button'
import { deleteAsset, renameAsset } from '@/lib/api/assets'
import { IMAGE_EXTENSIONS } from '@/lib/config'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Check, FileText, Pencil, Trash2, X } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { AssetPreviewTooltip } from './AssetPreviewTooltip'

const imageExtensions = IMAGE_EXTENSIONS

type Props = {
  pageId: string
  filename: string
  editingFilename: string | null
  setEditingFilename: (filename: string | null) => void
  onAssetVersionChange?: () => void
  onReload: () => void
  onInsert: (md: string) => void
  onFilenameChange?: (before: string, after: string) => void
}

export function AssetItem({
  pageId,
  filename,
  editingFilename,
  setEditingFilename,
  onReload,
  onInsert,
  onFilenameChange,
  onAssetVersionChange,
}: Props) {
  const assetUrl = filename
  const ext = filename.split('.').pop()?.toLowerCase()
  const isImage = imageExtensions.includes(ext ?? '')
  const baseName = filename.split('/').pop() ?? filename
  const isEditing = editingFilename === filename
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  const [newName, setNewName] = useState(baseName.replace(/\.[^/.]+$/, ''))

  const handleRename = useCallback(async () => {
    try {
      const newFilename = `${newName}.${ext}`
      if (newFilename === baseName) {
        setEditingFilename(null)
        return
      }

      await renameAsset(pageId, baseName, newFilename)
      toast.success('Asset renamed')
      onFilenameChange?.(baseName, newFilename)
      if (onAssetVersionChange) {
        onAssetVersionChange()
      }
      onReload()
    } catch (err: unknown) {
      if (err instanceof Error) {
        toast.error(`Rename failed: ${err.message}`)
        console.error('Rename failed', err)
      } else {
        if (typeof err === 'object' && err !== null && 'error' in err) {
          toast.error(`Rename failed: ${(err as { error: string }).error}`)
        }
      }
    }
  }, [
    pageId,
    baseName,
    newName,
    ext,
    onReload,
    onFilenameChange,
    onAssetVersionChange,
    setEditingFilename,
  ])

  const handleDelete = async () => {
    try {
      await deleteAsset(pageId, baseName)
      toast.success('Asset deleted')
      onReload()
      if (onAssetVersionChange) {
        onAssetVersionChange()
      }
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

  useEffect(() => {
    if (!isEditing) return

    const enterHotkey: HotKeyDefinition = {
      keyCombo: 'Enter',
      enabled: true,
      mode: ['dialog'],
      action: () => {
        handleRename()
      },
    }
    registerHotkey(enterHotkey)

    const escapeHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['dialog'],
      action: () => {
        setEditingFilename(null)
        setNewName(baseName.replace(/\.[^/.]+$/, ''))
      },
    }
    registerHotkey(escapeHotkey)

    return () => {
      unregisterHotkey(enterHotkey.keyCombo)
      unregisterHotkey(escapeHotkey.keyCombo)
    }

  }, [
    isEditing,
    baseName,
    registerHotkey,
    unregisterHotkey,
    handleRename,
    setEditingFilename,
  ])

  return (
    <li
      className="group flex items-center justify-between gap-2 rounded-md px-2 py-1 transition hover:bg-gray-100"
      onDoubleClick={handleInsertMarkdown}
      data-testid="asset-item"
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
            className="w-full border-b border-gray-300 bg-transparent text-sm text-gray-800 focus:outline-hidden"
          />
        ) : (
          <span className="cursor-pointer truncate text-sm text-gray-800 hover:underline">
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
              setEditingFilename(null)
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
            setEditingFilename(filename)
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
